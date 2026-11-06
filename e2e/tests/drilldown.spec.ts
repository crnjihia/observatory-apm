// path: e2e/tests/drilldown.spec.ts
import { test, expect } from '@playwright/test';

test.describe('Observatory APM Drill-down Telemetry Path', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
  });

  test('should render dashboard brand, KPIs, and Nairobi EAT timezone badge', async ({ page }) => {
    // 1. Verify Page Title and Branding
    await expect(page).toHaveTitle(/Observatory APM/);
    await expect(page.getByText('Observatory', { exact: false })).toBeVisible();

    // 2. Verify Nairobi EAT Timezone clock badge
    const timezoneBadge = page.getByText(/EAT \(UTC\+3\)/i);
    await expect(timezoneBadge).toBeVisible();

    // 3. Verify KPI Summary Cards
    await expect(page.getByText('Throughput', { exact: true })).toBeVisible();
    await expect(page.getByText('P50 Latency', { exact: true })).toBeVisible();
    await expect(page.getByText('P95 Latency', { exact: true })).toBeVisible();
    await expect(page.getByText('P99 (SLA Peak)', { exact: true })).toBeVisible();
    await expect(page.getByText('Error Rate', { exact: true })).toBeVisible();
  });

  test('should display 2D latency distribution heatmap and percentile overlays', async ({ page }) => {
    // 1. Verify Heatmap Card
    const heatmapHeader = page.getByText('Latency Distribution Heatmap');
    await expect(heatmapHeader).toBeVisible();

    // 2. Verify Percentile Curves Toggle
    const curvesCheckbox = page.getByLabel('Percentile Curves');
    await expect(curvesCheckbox).toBeVisible();
    await expect(curvesCheckbox).toBeChecked();

    // 3. Verify P50/P95/P99 legend items
    await expect(page.getByText('P50', { exact: true })).toBeVisible();
    await expect(page.getByText('P95', { exact: true })).toBeVisible();
    await expect(page.getByText('P99', { exact: true })).toBeVisible();

    // 4. Verify Heatmap SVG renders
    const heatmapSvg = page.locator('svg').filter({ hasText: /Time \(Nairobi EAT UTC\+3\)/ });
    await expect(heatmapSvg).toBeVisible();
  });

  test('should open TraceModal on SLA breach inspect or error drill-down', async ({ page }) => {
    // Look for "Inspect Spans" button or "Drill Down" in the error signatures table
    const inspectButton = page.getByRole('button', { name: /Inspect Spans|Drill Down/i }).first();
    await expect(inspectButton).toBeVisible();
    await inspectButton.click();

    // Verify TraceModal appears
    const modalHeader = page.getByText(/Trace Drill-Down:/i);
    await expect(modalHeader).toBeVisible();

    // Verify Tab controls
    await expect(page.getByRole('button', { name: 'Waterfall Timeline' })).toBeVisible();
    await expect(page.getByRole('button', { name: /Correlated Logs/i })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Raw JSON' })).toBeVisible();
  });

  test('should display distributed trace waterfall with timeline Gantt bars', async ({ page }) => {
    // Open modal
    const inspectButton = page.getByRole('button', { name: /Inspect Spans|Drill Down/i }).first();
    await inspectButton.click();

    // Verify Waterfall view
    await expect(page.getByText('Distributed Trace Waterfall')).toBeVisible();

    // Verify operations tree
    await expect(page.getByText('Operation & Service Hierarchy')).toBeVisible();

    // Check span row selection details
    const rootSpanRow = page.getByText('HTTP POST /v1/callbacks/mpesa').first();
    if (await rootSpanRow.isVisible()) {
      await rootSpanRow.click();
      // Should show Span ID, Parent ID, Start Time
      await expect(page.getByText('SPAN ID')).toBeVisible();
      await expect(page.getByText('START TIME')).toBeVisible();
    }
  });

  test('should switch to correlated logs tab and allow filtering', async ({ page }) => {
    // Open modal
    const inspectButton = page.getByRole('button', { name: /Inspect Spans|Drill Down/i }).first();
    await inspectButton.click();

    // Click Correlated Logs tab
    const logsTab = page.getByRole('button', { name: /Correlated Logs/i });
    await logsTab.click();

    // Verify Correlated Application Logs header
    await expect(page.getByText('Correlated Application Logs')).toBeVisible();

    // Verify severity filter buttons
    await expect(page.getByRole('button', { name: 'ALL', exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'ERROR', exact: true })).toBeVisible();

    // Verify log search box
    const searchInput = page.getByPlaceholder('Search logs...');
    await expect(searchInput).toBeVisible();
    await searchInput.fill('Daraja');

    // Close modal on Escape
    await page.keyboard.press('Escape');
    await expect(page.getByText(/Trace Drill-Down:/i)).not.toBeVisible();
  });

  test('should navigate to on-call simulator and inject incident', async ({ page }) => {
    await page.goto('/oncall');

    // 1. Verify On-Call Engineer Card
    await expect(page.getByText('Active Primary On-Call Engineer')).toBeVisible();
    await expect(page.getByText('Christopher Njihia')).toBeVisible();

    // 2. Verify Fault Injection Simulator scenarios
    await expect(page.getByText('Kenyan Infrastructure Fault & Anomaly Simulator')).toBeVisible();
    await expect(page.getByText('Safaricom Daraja 504 Gateway Timeout Spike')).toBeVisible();

    // 3. Inject Daraja 504 Timeout incident
    const injectBtn = page.getByRole('button', { name: 'Inject Incident' }).first();
    await injectBtn.click();

    // 4. Verify Success Banner and Incident in Log
    await expect(page.getByText(/injected successfully/i)).toBeVisible();
    await expect(page.getByRole('button', { name: 'Ack' }).first()).toBeVisible();

    // 5. Acknowledge the incident
    const ackBtn = page.getByRole('button', { name: 'Ack' }).first();
    await ackBtn.click();
    await expect(page.getByRole('button', { name: 'Resolve' }).first()).toBeVisible();
  });
});
