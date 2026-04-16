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
      !line.includes('team automation is disabled') &&
      !line.includes('status of 429') &&
      !line.includes('status code 429') &&
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
    await page.goto('/console/team-automation');
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
      await page.goto('/console/team-automation');
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

test('team automation page renders key controls without white screen', async ({
  page,
}) => {
  await loginAsAdmin(page);
  const errors = attachErrorCollectors(page);

  await page.goto('/console/team-automation');
  await page.waitForTimeout(1200);
  if (await isLikelyLoginPage(page)) {
    await loginAsAdmin(page);
    await page.goto('/console/team-automation');
    await page.waitForTimeout(1200);
  }

  const disabledHint = page.getByText(
    /team automation is disabled|connector disabled|base url not configured/i,
  );
  if (await disabledHint.first().isVisible().catch(() => false)) {
    await expect(disabledHint.first()).toBeVisible();
    const settingsLink = page
      .getByTestId('team-automation-open-setting')
      .locator('a')
      .first();
    if (await settingsLink.isVisible().catch(() => false)) {
      await expect(settingsLink).toHaveAttribute(
        'href',
        /\/console\/setting\?tab=team-automation/,
      );
    }
    await assertNoRuntimeErrors(errors);
    return;
  }

  await expect(
    page.getByRole('heading', { name: /Automation 控制台|Team Automation/i }),
  ).toBeVisible();
  await expect(page.getByTestId('team-automation-run-module')).toBeVisible();
  await expect(page.getByTestId('team-automation-run-account-status')).toBeVisible();
  await expect(page.getByTestId('team-automation-capability-source')).toBeVisible();

  const moduleSelect = page.getByTestId('team-automation-run-module').first();
  await moduleSelect.click();
  const redeemModuleOption = page
    .locator('.semi-select-option')
    .filter({ hasText: /Team 兑换|redeem/i })
    .first();
  if ((await redeemModuleOption.count()) > 0) {
    await redeemModuleOption.click();
  } else {
    await page.keyboard.press('Escape');
  }

  const redeemSiteField = page.getByTestId('team-automation-field-redeem_site');
  if ((await redeemSiteField.count()) > 0) {
    await expect(redeemSiteField).toBeVisible();
    const input = redeemSiteField.locator('input').first();
    if ((await input.count()) > 0) {
      await input.fill('https://helloteam.store');
    }
  }

  await assertNoRuntimeErrors(errors);
});

test('team automation capabilities endpoint returns structured payload', async ({
  page,
}) => {
  await loginAsAdmin(page);

  const userId = await readCurrentUserId(page);

  const resp = await requestWith429Retry(
    page.request,
    '/api/team-automation/capabilities',
    {
      headers: buildAdminHeaders(userId),
    },
  );
  const payload = await resp.json();
  if (!resp.ok() || !payload?.success) {
    const disabledMessage = String(payload?.message || '');
    expect(disabledMessage.toLowerCase()).toMatch(
      /team automation is disabled|connector disabled|base url not configured|未登录|无权进行此操作/,
    );
    return;
  }
  expect(Array.isArray(payload?.data?.modules)).toBeTruthy();
  expect(Array.isArray(payload?.data?.redeem_sites)).toBeTruthy();
  expect(typeof payload?.data?.supports).toBe('object');
  const siteValues = (payload?.data?.redeem_sites || [])
    .map((item) => String(item?.value || item || '').toLowerCase().trim())
    .filter(Boolean);
  expect(siteValues).toContain('helloteam.store');
  expect(siteValues).toContain('teamxz.store');
});
