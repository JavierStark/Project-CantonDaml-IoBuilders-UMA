import { test, expect } from '@playwright/test';

// Use timestamps to create unique party names between test runs
const ts = Date.now();

test.describe('Parties Management', () => {

  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('[data-testid="statusBadge"]')).toContainText(/Factory ready|Connected|✅/i, { timeout: 60000 });
  });

  test('should navigate to parties page and show pre-created parties', async ({ page }) => {
    await page.locator('[data-testid="nav-parties"]').click();
    await expect(page.locator('[data-testid="page-parties"]')).toBeVisible();

    const partyList = page.locator('[data-testid="partyList"]');
    await expect(partyList).toBeVisible();

    // Should show the pre-created parties
    await expect(partyList).toContainText('admin', { timeout: 5000 });
    await expect(partyList).toContainText('alice');
    await expect(partyList).toContainText('bob');
    await expect(partyList).toContainText('charlie');
  });

  test('should create a new party on participant2', async ({ page }) => {
    const partyName = `eve${ts}`;
    await page.locator('[data-testid="nav-parties"]').click();
    await expect(page.locator('[data-testid="page-parties"]')).toBeVisible();

    await page.locator('[data-testid="partyParticipant"]').selectOption('participant2');
    await page.locator('[data-testid="partyHint"]').fill(partyName);
    await page.locator('[data-testid="partySubmit"]').click();

    const result = page.locator('[data-testid="partyResult"]');
    // Party creation might succeed or say "already exists" from a previous run
    await expect(result).toContainText(/✅|Party created|already exists|already allocated/i, { timeout: 10000 });

    const partyList = page.locator('[data-testid="partyList"]');
    await expect(partyList).toContainText(partyName, { timeout: 5000 });
  });

  test('should create a new party on participant3', async ({ page }) => {
    const partyName = `mallory${ts}`;
    await page.locator('[data-testid="nav-parties"]').click();
    await expect(page.locator('[data-testid="page-parties"]')).toBeVisible();

    await page.locator('[data-testid="partyParticipant"]').selectOption('participant3');
    await page.locator('[data-testid="partyHint"]').fill(partyName);
    await page.locator('[data-testid="partySubmit"]').click();

    const result = page.locator('[data-testid="partyResult"]');
    await expect(result).toContainText(/✅|Party created|already exists|already allocated/i, { timeout: 10000 });

    const partyList = page.locator('[data-testid="partyList"]');
    await expect(partyList).toContainText(partyName, { timeout: 5000 });
  });

  test('should show new parties in select dropdowns across pages', async ({ page }) => {
    const partyName = `frank${ts}`;
    // Create a party first
    await page.locator('[data-testid="nav-parties"]').click();
    await page.locator('[data-testid="partyParticipant"]').selectOption('participant2');
    await page.locator('[data-testid="partyHint"]').fill(partyName);
    await page.locator('[data-testid="partySubmit"]').click();
    await expect(page.locator('[data-testid="partyResult"]')).toContainText(/✅|Party created|already exists|already allocated/i, { timeout: 10000 });

    // Navigate to mint page and reload to pick up the new party
    await page.locator('[data-testid="nav-mint"]').click();
    await page.waitForTimeout(500);
    await page.reload();
    await expect(page.locator('[data-testid="page-mint"]')).toBeVisible({ timeout: 10000 });
    const mintAdmin = page.locator('[data-testid="mintAdmin"]');
    await expect(mintAdmin.locator(`option[value="${partyName}"]`)).toBeVisible({ timeout: 5000 });
    const options = await mintAdmin.locator('option').allTextContents();
    expect(options).toContain(partyName);
  });
});
