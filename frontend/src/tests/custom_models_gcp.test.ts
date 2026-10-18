import { describe, it } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

describe('Custom Models GCP Integration & Refresh Top Bar Invariants', () => {
  const __dirname = path.dirname(fileURLToPath(import.meta.url))
  const pagePath = path.resolve(__dirname, '../pages/CustomModelsPage.tsx')
  const iconsPath = path.resolve(__dirname, '../components/AcpBrandIcons.tsx')
  const typesPath = path.resolve(__dirname, '../types.ts')

  const pageContent = fs.readFileSync(pagePath, 'utf8')
  const iconsContent = fs.readFileSync(iconsPath, 'utf8')
  const typesContent = fs.readFileSync(typesPath, 'utf8')

  const emojiRegex = /[\u{1F300}-\u{1F9FF}\u{2600}-\u{26FF}\u{2700}-\u{27BF}]/u

  it('guarantees zero decorative emojis in CustomModelsPage and AcpBrandIcons', () => {
    assert.strictEqual(emojiRegex.test(pageContent), false, 'CustomModelsPage must not contain decorative emojis')
    assert.strictEqual(emojiRegex.test(iconsContent), false, 'AcpBrandIcons must not contain decorative emojis')
  })

  it('top bar action portal has single icon-only refresh button placed after Add GCP Models', () => {
    assert.ok(pageContent.includes('handleRefreshAllQuotas'), 'Must call handleRefreshAllQuotas')
    assert.ok(
      pageContent.includes("className={refreshingQuotas ? 'spin' : ''}"),
      'Refresh icon must spin continuously when refreshingQuotas is true'
    )
    assert.ok(
      pageContent.includes('title="Refresh Quotas"'),
      'Single icon-only refresh button must have title Refresh Quotas'
    )
    // Verify there is only one refresh button in the top bar actions portal
    const portalActionsMatch = pageContent.match(/createPortal\(\s*<div[^>]*>([\s\S]*?)<\/div>,\s*portalTarget\s*\)/)
    assert.ok(portalActionsMatch, 'Top bar actions portal must exist')
    const portalInner = portalActionsMatch[1]
    const refreshBtnCount = (portalInner.match(/handleRefreshAllQuotas/g) || []).length
    assert.strictEqual(refreshBtnCount, 1, 'Top bar actions must contain exactly 1 refresh button')
    assert.ok(!portalInner.includes('Reload Models'), 'Duplicate Reload Models button must be removed')
  })

  it('Add GCP Models button is placed between Add Custom Model and the refresh button', () => {
    const portalActionsMatch = pageContent.match(/createPortal\(\s*<div[^>]*>([\s\S]*?)<\/div>,\s*portalTarget\s*\)/)
    assert.ok(portalActionsMatch)
    const portalInner = portalActionsMatch[1]
    const idxAddCustom = portalInner.indexOf('Add Custom Model')
    const idxAddGcp = portalInner.indexOf('Add GCP Models')
    const idxRefresh = portalInner.indexOf('handleRefreshAllQuotas')

    assert.ok(idxAddCustom !== -1, 'Add Custom Model button must exist')
    assert.ok(idxAddGcp !== -1, 'Add GCP Models button must exist')
    assert.ok(idxRefresh !== -1, 'Refresh Quotas button must exist')
    assert.ok(idxAddCustom < idxAddGcp, 'Add Custom Model must appear before Add GCP Models')
    assert.ok(idxAddGcp < idxRefresh, 'Add GCP Models must appear before Refresh Quotas button')
    assert.ok(portalInner.includes('<GoogleCloudLogoIcon size={15} />'), 'Add GCP Models button must use GoogleCloudLogoIcon')
  })

  it('AcpBrandIcons exports GoogleCloudLogoIcon using official GCP logo asset', () => {
    assert.ok(iconsContent.includes('export const GoogleCloudLogoIcon: React.FC<AcpBrandIconProps>'))
    assert.ok(iconsContent.includes('gcpLogo'), 'Must import official GCP logo asset')
    assert.ok(iconsContent.includes('alt="Google Cloud Platform"'), 'Must have proper alt text')
  })

  it('types.ts includes gcp in ProviderType and GCP credentials in CustomModel', () => {
    assert.ok(typesContent.includes("'gcp'"), "ProviderType union must include 'gcp'")
    assert.ok(typesContent.includes('gcp_project_id?: string'), 'CustomModel must include gcp_project_id')
    assert.ok(typesContent.includes('gcp_region?: string'), 'CustomModel must include gcp_region')
    assert.ok(typesContent.includes('gcp_service_account_key?: string'), 'CustomModel must include gcp_service_account_key')
    assert.ok(typesContent.includes('last_test_success?: boolean'), 'CustomModel must include last_test_success for the default-model gate')
    assert.ok(typesContent.includes('last_tested_at?: string'), 'CustomModel must include last_tested_at for the default-model gate')
  })

  it('GCP modal includes JSON key parsing, manual Model ID, Model Garden link, and updated instructions', () => {
    assert.ok(pageContent.includes('isGcpModalOpen'), 'GCP modal state must exist')
    assert.ok(pageContent.includes('handleGcpFileUpload'), 'File upload handler must exist')
    assert.ok(pageContent.includes('handleGcpJsonChange'), 'JSON text change and parse handler must exist')
    assert.ok(pageContent.includes('Agent Platform User'), 'GCP console setup instructions must mention Agent Platform User role')
    assert.ok(pageContent.includes('iam.disableServiceAccountKeyCreation'), 'Instructions must mention disableServiceAccountKeyCreation policy')
    assert.ok(pageContent.includes('console.cloud.google.com/agent-platform/model-garden'), 'Must link to Google Cloud Model Garden')
    assert.ok(pageContent.includes('handleTestGcpConnection'), 'Test connection handler for GCP must exist')
    assert.ok(pageContent.includes('handleSaveGcpModel'), 'Save GCP model handler must exist')
  })

  it('quota progress ring defaults untracked models to empty grey ring with N/A', () => {
    // Untracked / unverified models must never default to 100%
    assert.ok(!pageContent.includes("gaugePct = 100\n            emptyGrey = false\n            gaugeTitle = 'Untracked'"))
    assert.ok(pageContent.includes("gaugePct = null"))
    assert.ok(pageContent.includes("emptyGrey = true"))
  })

  it('renders official Google Cloud logo on GCP model cards (R2)', () => {
    assert.ok(
      pageContent.includes("m.provider_type === 'gcp'") && pageContent.includes('<GoogleCloudLogoIcon size={16}'),
      'GCP model cards must render GoogleCloudLogoIcon'
    )
  })

  it('displays GCP credit balance in connection test feedback (R5)', () => {
    assert.ok(
      pageContent.includes('GCP Credit Balance:'),
      'GCP test connection feedback must display GCP credit balance'
    )
  })

  it('openEditGcpModal extracts project_id from base_url when model.gcp_project_id is empty', () => {
    assert.ok(
      pageContent.includes("model.base_url.match(/\\/projects\\/([^/]+)/)"),
      'Must extract project ID from base_url when gcp_project_id is missing'
    )
  })

  it('handleTestGcpConnection configures draftGcpModel with quota_type balance', () => {
    assert.ok(
      pageContent.includes("quota_type: 'balance'"),
      'draftGcpModel must use quota_type balance'
    )
    assert.ok(
      pageContent.includes("balance_value: 'Active (GCP Billing)'"),
      'draftGcpModel must set balance_value to Active (GCP Billing)'
    )
  })

  it('enforces manual Model ID entry with zero preset buttons or dropdowns in GCP modal', () => {
    // Model ID input is an input field linking out to Google Cloud Model Garden
    assert.ok(pageContent.includes('placeholder="Enter Model ID (e.g. gemini-4-argon or meta/llama-3.3-70b-instruct)"'))
    assert.ok(pageContent.includes('console.cloud.google.com/agent-platform/model-garden'))
  })

  it('removes the Vertex AI location selector and derives the location from the model id', () => {
    // (a) the location control is gone from the modal
    assert.ok(!pageContent.includes('Vertex AI Location'), 'Vertex AI Location label must be removed')
    assert.ok(!pageContent.includes('<option value="global">'), 'Location <option> elements must be removed')
    assert.ok(!pageContent.includes('<option value="us-east5">'), 'Location <option> elements must be removed')
    assert.ok(!pageContent.includes('gcpRegion'), 'gcpRegion state and selector wiring must be removed')

    // (b) location is derived: Claude -> us-east5, everything else -> global, edit keeps stored location
    const resolveMatch = pageContent.match(/const resolveGcpRegion = [\s\S]*?\n\}/)
    assert.ok(resolveMatch, 'resolveGcpRegion helper must exist')
    const resolveBody = resolveMatch![0]
    assert.ok(resolveBody.includes('editing?.gcp_region'), 'Editing an existing model must keep its stored location')
    assert.ok(
      resolveBody.includes("modelId.toLowerCase().includes('claude') ? 'us-east5' : 'global'"),
      'Claude models must derive us-east5 and every other model must derive global'
    )

    // Both the test and save paths must send the derived location to the backend
    const regionUsages = pageContent.match(/base_url: gcpBaseUrlForRegion\(region\)/g) || []
    assert.strictEqual(regionUsages.length, 2, 'Test and save payloads must both use the derived region')
    const regionFields = pageContent.match(/gcp_region: region/g) || []
    assert.strictEqual(regionFields.length, 2, 'Test and save payloads must both send the derived region')
  })

  it('global location builds the region-less aiplatform.googleapis.com base URL', () => {
    assert.ok(
      pageContent.includes("region === 'global' ? 'https://aiplatform.googleapis.com' : `https://${region}-aiplatform.googleapis.com`"),
      'gcpBaseUrlForRegion must map global to the region-less host'
    )
  })

  it('renders a single error surface when a GCP connection test fails', () => {
    // (c) a failed test must render only gcpTestResult, never gcpError as well
    assert.ok(!pageContent.includes('GCP Vertex AI test failed'), 'Test failures must not be duplicated into gcpError')
    assert.ok(!pageContent.includes('GCP test connection failed'), 'Thrown test errors must not be duplicated into gcpError')
    assert.ok(!pageContent.includes('Switch to global'), 'The dead Switch to global action must be removed')
    assert.ok(
      !pageContent.includes('gcpTestResult.status_code === 404'),
      'The frontend 404 hint block must be removed (the backend already appends the global hint)'
    )

    const testFnMatch = pageContent.match(/const handleTestGcpConnection = async \(\) => \{[\s\S]*?\n  \}/)
    assert.ok(testFnMatch, 'handleTestGcpConnection must exist')
    const testFnBody = testFnMatch![0]
    // Everything from the request draft onwards is the network path: it must only ever
    // surface failures through gcpTestResult.
    const networkPath = testFnBody.slice(testFnBody.indexOf('const draftGcpModel'))
    assert.ok(networkPath.length > 0, 'Connection test draft payload must exist')
    assert.ok(!networkPath.includes('setGcpError('), 'The network test path must not set gcpError')
    assert.ok(
      testFnBody.includes("setGcpError('Service account JSON key is required to test GCP connection.')"),
      'Pre-flight validation must still use gcpError'
    )
    assert.ok(
      pageContent.includes('Connection test failed: ${gcpTestResult.message}'),
      'gcpTestResult must remain the single failure surface for connection tests'
    )
  })

  it('blocks setting a model as default while its last connection test failed', () => {
    assert.ok(
      pageContent.includes('Cannot set this model as the default while its last connection test failed. Run a successful test first.'),
      'Save handlers must refuse default promotion after a failed test'
    )
    assert.ok(
      pageContent.includes('last_test_success: gcpTestResult ? gcpTestResult.success : gcpEditingModel?.last_test_success'),
      'GCP save payload must carry the latest test outcome'
    )
    assert.ok(
      pageContent.includes('last_test_success: modalTestResult ? modalTestResult.success : editingModel?.last_test_success'),
      'Generic modal save payload must carry the latest test outcome'
    )
    assert.ok(
      pageContent.includes('The last connection test failed'),
      'GCP modal default toggle must explain why it is disabled after a failed test'
    )
  })

  it('preserves gemini-4-argon without forced rollback and supports default toggle in GCP modal', () => {
    assert.ok(!pageContent.includes("model.name === 'gemini-4-argon' ? 'gemini-2.5-flash'"), 'Must not rollback gemini-4-argon to gemini-2.5-flash')
    assert.ok(pageContent.includes('setGcpIsDefault'), 'GCP modal must track default custom model state')
    assert.ok(pageContent.includes('Set as default custom model'), 'GCP modal must offer Set as default custom model toggle')
  })

  it('GCP models expose manual balance and budget entry (manual-entry only, no auto probe fill)', () => {
    assert.ok(pageContent.includes('gcpBalanceValue'), 'GCP modal must track a manual balance value')
    assert.ok(pageContent.includes('Current Balance:'), 'GCP modal must expose a Current Balance field')
    assert.ok(pageContent.includes('setGcpBudgetCapType'), 'GCP modal must track a budget cap type')
    assert.ok(pageContent.includes('Budget Cap:'), 'GCP modal must expose a Budget Cap selector')
    assert.ok(pageContent.includes('(manual entry only)'), 'GCP quota section must state manual-entry only')
    assert.ok(pageContent.includes('quota_manual_override: true'), 'GCP save must flag the manual override')
    assert.ok(
      !pageContent.includes('quota_value: gcpTestResult?.quota_result'),
      'GCP save must not auto-fill balance/quota from the connection-test probe'
    )
    assert.ok(
      !pageContent.includes('balance_value: gcpTestResult?.quota_result'),
      'GCP save must not auto-fill balance_value from the connection-test probe'
    )
  })

  it('Quota & Price and GCP Quota & Budget sections default to collapsed with independent state', () => {
    assert.ok(
      pageContent.includes('const [isQuotaPriceOpen, setIsQuotaPriceOpen] = useState<boolean>(false)'),
      'Quota & Price section must default to collapsed'
    )
    assert.ok(
      pageContent.includes('const [isGcpQuotaBudgetOpen, setIsGcpQuotaBudgetOpen] = useState<boolean>(false)'),
      'GCP Quota & Budget section must default to collapsed with its own state'
    )
    assert.ok(
      !pageContent.includes('setIsQuotaPriceOpen(true)'),
      'Modal openers must not force the Quota section back open (user expansion persists for the session)'
    )
  })

  it('normalizes a GCP budget cap without a value back to none', () => {
    assert.ok(pageContent.includes('gcpHasBudgetCap'), 'GCP save must normalize cap type + value together')
    assert.ok(
      pageContent.includes("budget_cap_type: gcpHasBudgetCap ? gcpBudgetCapType : 'none'"),
      'Cap type must fall back to none when no value is provided'
    )
    assert.ok(
      pageContent.includes('budget_cap_value: gcpHasBudgetCap ? gcpBudgetCapValue : null'),
      'Cap value must be nulled when the cap is normalized away'
    )
  })

  it('removes the Null — Manual Entry Required badge', () => {
    assert.ok(!pageContent.includes('Manual Entry Required'), 'The yellow null-price pill must be removed')
  })
})
