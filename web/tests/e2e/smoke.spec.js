import { expect, test } from '@playwright/test';

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
    (line) => {
      if (!line) return false;
      if (line.includes('favicon')) return false;
      if (line.includes('ERR_BLOCKED_BY_CLIENT')) return false;
      if (line.startsWith('Warning:')) return false;
      const normalized = line.toLowerCase();
      if (
        normalized.includes('status of 429') ||
        normalized.includes('status code 429') ||
        (normalized.includes('failed to load resource') &&
          normalized.includes('429'))
      ) {
        return false;
      }
      return true;
    },
  );
  expect(filtered, `Unexpected runtime errors:\n${filtered.join('\n')}`).toEqual(
    [],
  );
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

    const style = getComputedStyle(el);
    let fg = parseRgb(style.color);
    let bgColor = style.backgroundColor;
    let bg = parseRgb(bgColor);

    let current = el;
    while (current && (!bg || bgColor.includes('0)'))) {
      current = current.parentElement;
      if (!current) break;
      const parentBg = getComputedStyle(current).backgroundColor;
      const parsed = parseRgb(parentBg);
      bgColor = parentBg;
      if (parsed && !parentBg.includes('0)')) {
        bg = parsed;
        break;
      }
    }

    if (!fg || !bg) return 0;
    return contrast(fg, bg);
  });

  expect(ratio).toBeGreaterThanOrEqual(minimum);
}

async function isSetupCompleted(page) {
  try {
    const res = await page.request.get('/api/status');
    if (!res.ok()) return false;
    const payload = await res.json();
    return Boolean(payload?.data?.setup);
  } catch (error) {
    return false;
  }
}

test('home page renders key actions', async ({ page }) => {
  test.skip(
    !(await isSetupCompleted(page)),
    'environment not initialized (setup=false), skip post-setup home assertions',
  );
  const errors = attachErrorCollectors(page);
  await page.goto('/');

  await expect(page).toHaveURL('/');
  const getApiKeyLink = page
    .getByRole('link', { name: /获取密钥|Get API Key/i })
    .first();
  const docsLink = page.getByRole('link', { name: /文档|Docs/i }).first();
  const consoleLink = page.getByRole('link', { name: /控制台|Console/i }).first();

  await expect(getApiKeyLink).toBeVisible();
  await expect(docsLink).toBeVisible();
  await expect(consoleLink).toBeVisible();
  await assertMinContrast(getApiKeyLink);

  await assertNoRuntimeErrors(errors);
});

test('login page renders form controls', async ({ page }) => {
  const errors = attachErrorCollectors(page);
  await page.goto('/login');

  await expect(page).toHaveURL('/login');
  await expect(page.locator('input')).toHaveCount(2, { timeout: 20_000 });

  const submitButton = page
    .getByRole('button', { name: /继续|Continue|登录|Log In/i })
    .first();
  await expect(submitButton).toBeVisible();

  await assertNoRuntimeErrors(errors);
});

test('docs page iframe loads mirrored docs content', async ({ page }) => {
  test.skip(
    !(await isSetupCompleted(page)),
    'environment not initialized (setup=false), skip docs assertions',
  );
  const errors = attachErrorCollectors(page);
  await page.goto('/docs');

  const iframe = page.locator('iframe[title="Docs"]');
  await expect(iframe).toBeVisible();

  const frame = await iframe.contentFrame();
  await expect(frame.locator('body')).toContainText(
    /Caowo 文档中心|Caowo Documentation/i,
  );
  await expect(frame.getByText(/Codex 一键配置|One-click Setup/i)).toBeVisible();
  const docsSetupButton = frame.locator('a.setup-banner-btn').first();
  await expect(docsSetupButton).toBeVisible();
  await assertMinContrast(docsSetupButton);
  const docsModelName = frame.locator('.model-name').first();
  if ((await docsModelName.count()) > 0) {
    await docsModelName.scrollIntoViewIfNeeded();
    await assertMinContrast(docsModelName);
  }
  const docsHotTag = frame.locator('.tag-hot').first();
  if ((await docsHotTag.count()) > 0) {
    await docsHotTag.scrollIntoViewIfNeeded();
    await assertMinContrast(docsHotTag);
  }
  const docsCodeTag = frame.locator('.tag-code').first();
  if ((await docsCodeTag.count()) > 0) {
    await docsCodeTag.scrollIntoViewIfNeeded();
    await assertMinContrast(docsCodeTag);
  }
  const docsActiveNav = frame.locator('a.nav-link.active').first();
  if ((await docsActiveNav.count()) > 0) {
    await assertMinContrast(docsActiveNav);
  }

  await assertNoRuntimeErrors(errors);
});

test('setup page iframe loads and contains console entry', async ({ page }) => {
  test.skip(
    !(await isSetupCompleted(page)),
    'environment not initialized (setup=false), skip setup-doc assertions',
  );
  const errors = attachErrorCollectors(page);
  await page.goto('/docs/setup');

  const iframe = page.locator('iframe[title="Setup"]');
  await expect(iframe).toBeVisible();

  const frame = await iframe.contentFrame();
  await expect(frame.locator('body')).toContainText(/一键配置|One-click Setup/i);
  await expect(frame.getByText(/控制台入口|Console/i)).toBeVisible();
  await expect(frame.getByText(/Worker Token/i)).toBeVisible();
  const generateButton = frame.locator('button.btn.btn-primary').first();
  await expect(generateButton).toBeVisible();
  await assertMinContrast(generateButton);
  await frame.locator('#apiKey').fill('sk-e2e-contrast-check');
  await generateButton.click();
  const commandBox = frame.locator('#cmdBox');
  await expect(commandBox).toBeVisible();
  await assertMinContrast(commandBox);

  await assertNoRuntimeErrors(errors);
});
