import { expect, test } from '@playwright/test';

const ADMIN_ACCESS_TOKEN = String(
  process.env.E2E_ADMIN_ACCESS_TOKEN || '',
).trim().replace(/^Bearer\s+/i, '');
const ADMIN_ACCESS_TOKEN_HEADER = ADMIN_ACCESS_TOKEN
  ? `Bearer ${ADMIN_ACCESS_TOKEN}`
  : '';
const ADMIN_FALLBACK_USER_ID = String(process.env.E2E_ADMIN_USER_ID || '1');

function buildAdminHeaders(userId = ADMIN_FALLBACK_USER_ID) {
  const headers = {
    'New-Api-User': String(userId || ADMIN_FALLBACK_USER_ID),
  };
  if (ADMIN_ACCESS_TOKEN_HEADER) {
    headers.Authorization = ADMIN_ACCESS_TOKEN_HEADER;
  }
  return headers;
}

async function readCurrentUserId(page) {
  return page.evaluate(() => {
    try {
      const raw = localStorage.getItem('user');
      const parsed = raw ? JSON.parse(raw) : {};
      return String(parsed?.id || parsed?.user?.id || '1');
    } catch {
      return '1';
    }
  });
}

async function isLikelyLoginPage(page) {
  if (/\/login(?:[/?#]|$)/i.test(page.url())) {
    return true;
  }
  const passwordInput = page.locator('input[type="password"]').first();
  return passwordInput.isVisible({ timeout: 800 }).catch(() => false);
}

function attachErrorCollectors(page) {
  const errors = [];
  page.on('pageerror', (err) => {
    errors.push(err.message || String(err));
  });
  page.on('console', (msg) => {
    if (msg.type() === 'error') {
      errors.push(msg.text());
    }
  });
  return errors;
}

async function assertNoRuntimeErrors(errors) {
  const filtered = errors.filter(
    (line) =>
      line &&
      !line.includes('favicon') &&
      !line.includes('ERR_BLOCKED_BY_CLIENT') &&
      !line.startsWith('Warning:'),
  );
  expect(filtered, `Unexpected runtime errors:\n${filtered.join('\n')}`).toEqual(
    [],
  );
}

async function loginAsAdmin(page) {
  const username = process.env.E2E_ADMIN_USER || process.env.E2E_ADMIN_USERNAME;
  const password = process.env.E2E_ADMIN_PASSWORD;
  const accessToken = ADMIN_ACCESS_TOKEN;
  const accessTokenHeader = ADMIN_ACCESS_TOKEN_HEADER;
  const accessTokenUserId = ADMIN_FALLBACK_USER_ID;

  test.skip(
    !accessToken && (!username || !password),
    'Missing E2E admin credentials (set E2E_ADMIN_ACCESS_TOKEN, or E2E_ADMIN_USER/E2E_ADMIN_PASSWORD)',
  );

  const isLoggedIn = async () => {
    await page.goto('/console/chatmock');
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(700);
    return !(await isLikelyLoginPage(page));
  };

  const hydrateUserState = async (userData) => {
    if (!userData || typeof userData !== 'object') return;
    await page.goto('/');
    await page.waitForLoadState('domcontentloaded');
    await page.evaluate((data) => {
      localStorage.setItem('user', JSON.stringify(data));
    }, userData);
  };

  const loginViaApi = async () => {
    for (let attempt = 1; attempt <= 6; attempt += 1) {
      let resp;
      try {
        resp = await page.request.post('/api/user/login', {
          data: {
            username,
            password,
          },
        });
      } catch {
        await page.waitForTimeout(800 * attempt);
        continue;
      }
      if (resp.status() === 429) {
        await page.waitForTimeout(1000 * attempt);
        continue;
      }
      const payload = await resp.json().catch(() => ({}));
      if (payload?.data?.require_2fa) {
        throw new Error('admin login requires 2fa, e2e account unsupported');
      }
      if (resp.ok() && payload?.success) {
        await hydrateUserState(payload?.data || null);
        if (await isLoggedIn()) {
          return true;
        }
      }
      await page.waitForTimeout(600 * attempt);
    }
    return false;
  };

  const loginViaAccessToken = async () => {
    if (!accessTokenHeader) return false;
    for (let attempt = 1; attempt <= 4; attempt += 1) {
      let resp;
      try {
        resp = await page.request.get('/api/user/self', {
          headers: buildAdminHeaders(accessTokenUserId),
        });
      } catch {
        await page.waitForTimeout(800 * attempt);
        continue;
      }
      if (resp.status() === 429) {
        await page.waitForTimeout(1200 * attempt);
        continue;
      }
      if (!resp.ok()) {
        await page.waitForTimeout(500 * attempt);
        continue;
      }
      const payload = await resp.json().catch(() => ({}));
      if (!payload?.success || !payload?.data?.id) {
        await page.waitForTimeout(500 * attempt);
        continue;
      }
      await hydrateUserState({
        ...payload.data,
        token: accessToken,
      });
      await page.goto('/console/chatmock');
      await page.waitForLoadState('domcontentloaded');
      await page.waitForTimeout(800);
      if (!(await isLikelyLoginPage(page))) {
        return true;
      }
    }
    return false;
  };

  if (await isLoggedIn()) {
    return;
  }

  if (await loginViaAccessToken()) {
    return;
  }

  if (await loginViaApi()) {
    return;
  }

  if (!username || !password) {
    throw new Error('admin login via access token failed and username/password not provided');
  }

  for (let attempt = 1; attempt <= 5; attempt += 1) {
    await page.goto('/login');
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(400);
    if (!/\/login(?:[/?#]|$)/i.test(page.url())) {
      return;
    }
    const passwordInput = page.locator('input[type="password"]').first();
    const formVisible = await passwordInput
      .isVisible({ timeout: 3_000 })
      .catch(() => false);
    if (!formVisible) {
      await page.waitForTimeout(600 * attempt);
      continue;
    }
    await page.locator('input').nth(0).fill('');
    await page.locator('input').nth(0).fill(username);
    await passwordInput.fill('');
    await passwordInput.fill(password);
    await page
      .getByRole('button', { name: /登录|Log In|继续|Continue/i })
      .first()
      .click();
    await page.waitForTimeout(1200 + attempt * 800);
    if (await isLoggedIn()) {
      return;
    }
    const limitHint = page.getByText(/请求次数过多|too many requests|rate limit/i);
    if (await limitHint.first().isVisible().catch(() => false)) {
      await page.waitForTimeout(2000 * attempt);
    }
  }

  throw new Error('admin login failed after retries');
}

async function requestWith429Retry(request, url, options = {}, maxAttempts = 8) {
  let response;
  for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
    response = await request.get(url, options);
    if (response.status() !== 429) {
      return response;
    }
    await new Promise((resolve) => setTimeout(resolve, 1200 * attempt));
  }
  return response;
}

async function assertMinContrast(locator, minimum = 4.5) {
  const ratio = await locator.evaluate((el) => {
    const parseRgb = (value) => {
      const match = value && value.match(/rgba?\(([^)]+)\)/i);
      if (!match) return null;
      const [r, g, b] = match[1]
        .split(',')
        .slice(0, 3)
        .map((v) => Number(v.trim()));
      if ([r, g, b].some((v) => Number.isNaN(v))) return null;
      return [r, g, b];
    };

    const luminance = ([r, g, b]) => {
      const norm = [r, g, b].map((v) => {
        const c = v / 255;
        return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
      });
      return 0.2126 * norm[0] + 0.7152 * norm[1] + 0.0722 * norm[2];
    };

    const contrast = (fg, bg) => {
      const l1 = luminance(fg);
      const l2 = luminance(bg);
      const bright = Math.max(l1, l2);
      const dark = Math.min(l1, l2);
      return (bright + 0.05) / (dark + 0.05);
    };

    const getBgColor = (el) => {
      let current = el;
      while (current) {
        const style = getComputedStyle(current);
        const parsed = parseRgb(style.backgroundColor);
        if (parsed) {
          const alphaMatch = style.backgroundColor.match(
            /rgba\([^,]+,[^,]+,[^,]+,\s*([0-9.]+)/i,
          );
          const alpha = alphaMatch ? Number(alphaMatch[1]) : 1;
          if (Number.isFinite(alpha) && alpha > 0.05) {
            return parsed;
          }
        }
        current = current.parentElement;
      }
      return [255, 255, 255];
    };

    const fg = parseRgb(getComputedStyle(el).color);
    const bg = getBgColor(el);
    if (!fg || !bg) return 0;
    return contrast(fg, bg);
  });

  expect(ratio).toBeGreaterThanOrEqual(minimum);
}

test('chatmock add modal and batch import render without white screen', async ({
  page,
}) => {
  await loginAsAdmin(page);
  const errors = attachErrorCollectors(page);

  await page.goto('/console/chatmock');
  await page.waitForTimeout(1200);
  await expect(page.getByTestId('chatmock-mode-basic')).toBeVisible();
  await expect(page.getByTestId('chatmock-mode-advanced')).toBeVisible();
  await expect(page.getByTestId('chatmock-sync-channel')).toBeVisible();
  await expect(page.getByTestId('chatmock-pool-channel-status')).toBeVisible();
  await expect(page.getByText(/默认系统提示词|default instructions/i).first()).toBeVisible();
  await expect(page.locator('textarea').first()).toBeVisible();
  await expect(page.getByTestId('chatmock-stabilize')).toBeVisible();
  await expect(page.getByText(/权益数据待刷新|stale entitlement/i).first()).toBeVisible();
  await expect(page.getByText(/重置后待回填|reset pending/i).first()).toBeVisible();
  await expect(page.getByText(/24h内授权到期|token expiring/i).first()).toBeVisible();
  await expect(page.getByText(/鉴权异常|auth/i).first()).toBeVisible();
  await expect(page.getByText(/额度问题|quota issue/i).first()).toBeVisible();
  await expect(page.getByText(/额度耗尽|exhausted/i).first()).toBeVisible();
  await expect(page.getByText(/Free当量\(7d\)|free equivalent/i).first()).toBeVisible();
  await expect(page.getByRole('columnheader', { name: /额度状态|quota status/i })).toBeVisible();
  await expect(page.getByRole('columnheader', { name: /路由评分|route score/i })).toBeVisible();
  const sortSelect = page.locator('.semi-select').filter({
    hasText: /默认排序|Default \(Priority\/Weight\)/i,
  });
  await expect(sortSelect.first()).toBeVisible();
  await expect(page.getByTestId('chatmock-export-inspector')).toHaveCount(0);
  const weeklyProgress = page.locator('[aria-label="weekly remaining progress"]').first();
  if ((await weeklyProgress.count()) > 0) {
    await expect(weeklyProgress).toBeVisible();
  }
  await expect(
    page.getByRole('columnheader', { name: /weekly剩余|weekly remain/i }),
  ).toBeVisible();
  await expect(
    page.getByRole('button', { name: /应用推荐参数|recommended/i }).first(),
  ).toBeVisible();
  await expect(page.getByTestId('chatmock-pool-preset-select')).toBeVisible();
  await expect(page.getByTestId('chatmock-pool-preset-apply')).toBeVisible();
  await expect(page.getByTestId('chatmock-pool-preset-apply-save')).toBeVisible();
  await page.getByTestId('chatmock-pool-preset-apply').click();
  await expect(page.getByTestId('chatmock-default-instructions')).toHaveValue(
    /production-grade coding assistant|quota-aware routing/i,
  );
  await expect(page.getByTestId('chatmock-smart-switch-sort')).toBeVisible();
  await page.getByTestId('chatmock-smart-switch-sort').click();
  const smartSwitchCandidate = page.getByTestId('chatmock-smart-switch-candidate');
  if ((await smartSwitchCandidate.count()) > 0) {
    await expect(smartSwitchCandidate.first()).toBeVisible();
  }
  await expect(page.getByTestId('chatmock-batch-health')).toHaveCount(0);
  await expect(page.getByTestId('chatmock-route-diagnose')).toHaveCount(0);
  await expect(page.getByTestId('chatmock-inspect-sync')).toHaveCount(0);

  await page.getByTestId('chatmock-add-account').click();
  await expect(page.locator('.semi-modal')).toBeVisible();
  const accountModalInputCount = await page.locator('.semi-modal input').count();
  expect(accountModalInputCount).toBeGreaterThanOrEqual(12);
  await expect(page.getByText(/^ID Token$/).first()).toBeVisible();
  await expect(page.getByText(/^Refresh Token$/).first()).toBeVisible();

  await page.keyboard.press('Escape');
  await page.waitForTimeout(300);

  await page.getByTestId('chatmock-batch-import').click();
  const batchModal = page.locator('.semi-modal:has-text("批量导入账号")');
  await expect(batchModal).toBeVisible();
  await expect(batchModal.locator('input[type="file"]')).toHaveAttribute(
    'multiple',
    '',
  );
  await expect(batchModal.locator('textarea')).toBeVisible();
  await batchModal
    .locator('textarea')
    .fill('[{"name":"e2e-invalid","account":"","api_key":"dummy-key"}]');
  await batchModal.locator('.semi-modal-footer button').last().click();
  await expect(batchModal.getByText(/导入结果：总计|Import result:/i)).toBeVisible();

  await page.keyboard.press('Escape');
  await page.waitForTimeout(300);

  await page.getByTestId('chatmock-mode-advanced').click();
  await page.waitForTimeout(300);
  const batchHealthCheckButton = page.getByTestId('chatmock-batch-health');
  const batchEntitlementCheckButton = page.getByTestId('chatmock-batch-entitlement');
  const batchRefreshAuthButton = page.getByTestId('chatmock-batch-refresh-token');
  const batchInspectButton = page.getByTestId('chatmock-inspect-all');
  const inspectSyncButton = page.getByTestId('chatmock-inspect-sync');
  const exportJsonButton = page.getByTestId('chatmock-export-json');
  const exportInspectorButton = page.getByTestId('chatmock-export-inspector');
  const routeDiagnoseButton = page.getByTestId('chatmock-route-diagnose');
  await expect(batchHealthCheckButton).toBeVisible();
  await expect(batchEntitlementCheckButton).toBeVisible();
  await expect(batchRefreshAuthButton).toBeVisible();
  await expect(batchInspectButton).toBeVisible();
  await expect(inspectSyncButton).toBeVisible();
  await expect(exportJsonButton).toBeVisible();
  await expect(exportInspectorButton).toBeVisible();
  await routeDiagnoseButton.click();
  const diagnoseModal = page.locator('.semi-modal:has-text("账号池路由诊断结果")');
  await expect(diagnoseModal).toBeVisible();
  await expect(diagnoseModal.getByText(/调度策略|selection policy/i)).toBeVisible();
  await diagnoseModal.getByRole('button', { name: /关闭|Close/i }).first().click();
  await page.waitForTimeout(300);

  await page.getByRole('tab', { name: /邮箱管理|mail/i }).first().click();
  const mailboxDisabledHint = page.getByText(
    /CHATMOCK_MAILBOX_ENABLED=false|邮箱中心未启用/i,
  );
  const mailboxCreateButton = page
    .getByRole('button', { name: /新增邮箱|Add Mailbox/i })
    .first();
  const mailboxBatchImportButton = page
    .getByRole('button', { name: /批量导入|Batch/i })
    .first();
  await expect(
    mailboxDisabledHint.or(mailboxCreateButton).or(mailboxBatchImportButton).first(),
  ).toBeVisible();

  if (await mailboxBatchImportButton.isVisible()) {
    await mailboxBatchImportButton.click();
    const mailboxBatchModal = page.locator('.semi-modal:has-text("批量导入邮箱")');
    await expect(mailboxBatchModal).toBeVisible();
    await expect(mailboxBatchModal.locator('input[type="file"]')).toHaveAttribute(
      'multiple',
      '',
    );
    const mailboxTemplateButton = mailboxBatchModal
      .getByRole('button', { name: /填充模板|Template/i })
      .first();
    if ((await mailboxTemplateButton.count()) > 0) {
      await expect(mailboxTemplateButton).toBeVisible();
    }
    await mailboxBatchModal
      .locator('textarea')
      .fill('[{"name":"mb-invalid","email":"demo@example.com","username":"demo@example.com"}]');
    await mailboxBatchModal.locator('.semi-modal-footer button').last().click();
    await expect(mailboxBatchModal.getByText(/导入结果：总计|Import result:/i)).toBeVisible();
    await page.keyboard.press('Escape');
    await page.waitForTimeout(300);
  }

  await assertNoRuntimeErrors(errors);
});

test('key pages have no obvious low-contrast foreground text', async ({ page }) => {
  await loginAsAdmin(page);

  await page.goto('/pricing');
  await page.waitForTimeout(1200);
  const pricingActionButton = page.locator('main button:not([disabled])').first();
  await expect(pricingActionButton).toBeVisible();
  await assertMinContrast(pricingActionButton);
  const pricingHeading = page.locator('main h1, main h2, main .semi-typography').first();
  if ((await pricingHeading.count()) > 0) {
    await assertMinContrast(pricingHeading);
  }

  await page.goto('/console/models');
  await page.waitForTimeout(1200);
  if (await isLikelyLoginPage(page)) {
    await loginAsAdmin(page);
    await page.goto('/console/models');
    await page.waitForTimeout(1200);
  }
  const addModelButton = page
    .getByRole('button', { name: /添加模型|Add Model/i })
    .first();
  await expect(addModelButton).toBeVisible();
  await assertMinContrast(addModelButton);
  const modelButtons = page.locator('main button:not([disabled])');
  const modelButtonCount = await modelButtons.count();
  if (modelButtonCount > 0) {
    await assertMinContrast(modelButtons.first());
  }

  await page.goto('/console/channel');
  await page.waitForTimeout(1200);
  if (await isLikelyLoginPage(page)) {
    await loginAsAdmin(page);
    await page.goto('/console/channel');
    await page.waitForTimeout(1200);
  }
  const addChannelButton = page
    .getByRole('button', { name: /添加渠道|新增渠道|Add Channel|Add/i })
    .first();
  if (
    (await addChannelButton.count()) > 0 &&
    (await addChannelButton.isVisible().catch(() => false))
  ) {
    await assertMinContrast(addChannelButton);
  } else {
    const fallbackChannelButton = page.locator('main button:not([disabled])').first();
    await expect(fallbackChannelButton).toBeVisible();
    await assertMinContrast(fallbackChannelButton);
  }
  const channelHeaderCell = page.locator('.semi-table-thead th').first();
  if ((await channelHeaderCell.count()) > 0) {
    await assertMinContrast(channelHeaderCell);
  }

  await page.goto('/console/user');
  await page.waitForTimeout(1200);
  if (await isLikelyLoginPage(page)) {
    await loginAsAdmin(page);
    await page.goto('/console/user');
    await page.waitForTimeout(1200);
  }
  const addUserButton = page
    .getByRole('button', { name: /添加用户|Add User/i })
    .first();
  if ((await addUserButton.count()) > 0) {
    await expect(addUserButton).toBeVisible();
    await assertMinContrast(addUserButton);
  }

  await page.goto('/console/token');
  await page.waitForTimeout(1200);
  if (await isLikelyLoginPage(page)) {
    await loginAsAdmin(page);
    await page.goto('/console/token');
    await page.waitForTimeout(1200);
  }
  const addTokenButton = page
    .getByRole('button', { name: /添加令牌|Add Token/i })
    .first();
  if ((await addTokenButton.count()) > 0) {
    await expect(addTokenButton).toBeVisible();
    await assertMinContrast(addTokenButton);
  }

  await page.goto('/console/chatmock');
  await page.waitForTimeout(1200);
  if (await isLikelyLoginPage(page)) {
    await loginAsAdmin(page);
    await page.goto('/console/chatmock');
    await page.waitForTimeout(1200);
  }
  await assertMinContrast(page.getByRole('button', { name: /刷新配置/i }).first());
  await assertMinContrast(page.getByRole('button', { name: /新增账号|Add/i }).first());
  const chatmockQuotaHeader = page
    .getByRole('columnheader', { name: /weekly剩余|weekly remain/i })
    .first();
  if ((await chatmockQuotaHeader.count()) > 0) {
    await assertMinContrast(chatmockQuotaHeader);
  }
  const chatmockHighQuotaTag = page
    .locator('.semi-tag')
    .filter({ hasText: /^high$/i })
    .first();
  if ((await chatmockHighQuotaTag.count()) > 0) {
    await assertMinContrast(chatmockHighQuotaTag);
  }
  const mailboxTab = page.getByRole('tab', { name: /邮箱管理|mail/i }).first();
  await mailboxTab.click();
  await page.waitForTimeout(800);
  const mailboxDisabledHint = page.getByText(
    /CHATMOCK_MAILBOX_ENABLED=false|邮箱中心未启用/i,
  );
  const mailboxBatchTestButton = page
    .getByRole('button', { name: /全量连接测试|test-all/i })
    .first();
  if (await mailboxDisabledHint.isVisible().catch(() => false)) {
    await expect(mailboxDisabledHint).toBeVisible();
  } else {
    await expect(mailboxBatchTestButton).toBeVisible();
    await assertMinContrast(mailboxBatchTestButton);
  }
});

test('chatmock mailbox API accepts IMAP-only payload (SMTP optional)', async ({
  page,
}) => {
  await loginAsAdmin(page);
  const userId = await readCurrentUserId(page);
  const headers = buildAdminHeaders(userId);
  const now = Date.now();
  const email = `imap-only-${now}@example.com`;

  const createResp = await page.request.post('/api/admin/chatmock/mailboxes', {
    headers,
    data: {
      name: `imap-only-${now}`,
      email,
      username: email,
      password: 'e2e-placeholder-pass',
      imap_host: 'imap.example.com',
      imap_port: 993,
      imap_security: 'ssl',
      status: 'disabled',
      poll_interval_second: 120,
    },
  });
  if (!createResp.ok()) {
    const failedPayload = await createResp.json().catch(() => ({}));
    const failedMessage = String(failedPayload?.message || '');
    test.skip(
      /CHATMOCK_MAILBOX_ENABLED=false|邮箱中心未启用|无权进行此操作/i.test(
        failedMessage,
      ),
      `mailbox API not available in current env: ${failedMessage || createResp.status()}`,
    );
  }
  expect(createResp.ok()).toBeTruthy();
  const createData = await createResp.json();
  expect(createData?.success).toBeTruthy();
  const mailboxId = createData?.data?.id;
  expect(Number(mailboxId)).toBeGreaterThan(0);
  expect(String(createData?.data?.smtp_host || '')).toBe('');

  const deleteResp = await page.request.delete(
    `/api/admin/chatmock/mailboxes/${mailboxId}`,
    {
      headers,
    },
  );
  expect(deleteResp.ok()).toBeTruthy();
  const deleteData = await deleteResp.json();
  expect(deleteData?.success).toBeTruthy();
});

test('chatmock one-click channel sync includes codex model coverage', async ({
  page,
}) => {
  await loginAsAdmin(page);
  await page.goto('/console/chatmock');
  await page.waitForTimeout(1200);

  await page
    .getByRole('button', { name: /一键接入 NewAPI 渠道|sync/i })
    .first()
    .click();
  await page.waitForTimeout(1800);

  const userId = await readCurrentUserId(page);

  const channelResp = await requestWith429Retry(
    page.request,
    '/api/channel/search?keyword=chatmock_pool&p=1&page_size=50',
    {
      headers: buildAdminHeaders(userId),
    },
  );
  expect(channelResp.ok()).toBeTruthy();
  const channelData = await channelResp.json();
  expect(channelData?.success).toBeTruthy();

  const channels = Array.isArray(channelData?.data?.items) ? channelData.data.items : [];
  const target = channels.find(
    (item) =>
      Number(item?.type) === 58 || String(item?.key || '').trim() === 'chatmock_pool',
  );
  expect(target).toBeTruthy();

  const models = String(target?.models || '')
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean);
  expect(models.length).toBeGreaterThanOrEqual(10);
  expect(models).toContain('gpt-5.1-codex');
  expect(models).toContain('gpt-5.1-codex-max');
  expect(models).toContain('gpt-5.3-codex');

  const statusResp = await requestWith429Retry(
    page.request,
    '/api/admin/chatmock/channel-status',
    {
      headers: buildAdminHeaders(userId),
    },
  );
  expect(statusResp.ok()).toBeTruthy();
  const statusData = await statusResp.json();
  expect(statusData?.success).toBeTruthy();
  expect(statusData?.data?.exists).toBeTruthy();
  expect(Number(statusData?.data?.channel_id)).toBeGreaterThan(0);
  expect(Number(statusData?.data?.models_count)).toBeGreaterThan(0);
});

test('chatmock observability APIs expose route logs and snapshots', async ({
  page,
}) => {
  await loginAsAdmin(page);

  const userId = await readCurrentUserId(page);

  const routeLogResp = await requestWith429Retry(
    page.request,
    '/api/admin/chatmock/route-logs?page=1&page_size=5',
    {
      headers: buildAdminHeaders(userId),
    },
  );
  expect(routeLogResp.ok()).toBeTruthy();
  const routeLogPayload = await routeLogResp.json();
  expect(routeLogPayload?.success).toBeTruthy();
  expect(Array.isArray(routeLogPayload?.data?.items)).toBeTruthy();

  const snapshotResp = await requestWith429Retry(
    page.request,
    '/api/admin/chatmock/account-snapshots?page=1&page_size=5',
    {
      headers: buildAdminHeaders(userId),
    },
  );
  expect(snapshotResp.ok()).toBeTruthy();
  const snapshotPayload = await snapshotResp.json();
  expect(snapshotPayload?.success).toBeTruthy();
  expect(Array.isArray(snapshotPayload?.data?.items)).toBeTruthy();

  const eventResp = await requestWith429Retry(
    page.request,
    '/api/admin/chatmock/mail-events?page=1&page_size=5',
    {
      headers: buildAdminHeaders(userId),
    },
  );
  expect(eventResp.ok()).toBeTruthy();
  const eventPayload = await eventResp.json();
  expect(eventPayload?.success).toBeTruthy();
  const events = Array.isArray(eventPayload?.data?.items)
    ? eventPayload.data.items
    : [];

  if (events.length > 0) {
    const firstID = Number(events[0]?.id || 0);
    if (firstID > 0) {
      const reparseResp = await page.request.post(
        `/api/admin/chatmock/mail-events/${firstID}/reparse`,
        {
          headers: buildAdminHeaders(userId),
        },
      );
      expect(reparseResp.ok()).toBeTruthy();
      const reparsePayload = await reparseResp.json();
      expect(reparsePayload?.success).toBeTruthy();
    }
  }
});
