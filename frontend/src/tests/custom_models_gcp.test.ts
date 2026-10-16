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
  })

  it('GCP modal includes JSON key parsing, region selection, manual Model ID, Model Garden link, and updated instructions', () => {
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
    assert.ok(pageContent.includes('placeholder="Enter Model ID (e.g. gemini-2.5-flash or meta/llama-3.3-70b-instruct)"'))
    assert.ok(pageContent.includes('console.cloud.google.com/agent-platform/model-garden'))
  })

  it('preserves gemini-4-argon without forced rollback and supports default toggle in GCP modal', () => {
    assert.ok(!pageContent.includes("model.name === 'gemini-4-argon' ? 'gemini-2.5-flash'"), 'Must not rollback gemini-4-argon to gemini-2.5-flash')
    assert.ok(pageContent.includes('setGcpIsDefault'), 'GCP modal must track default custom model state')
    assert.ok(pageContent.includes('Set as default custom model'), 'GCP modal must offer Set as default custom model toggle')
  })
})
