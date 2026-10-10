import { describe, it } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'

describe('Launch Antigravity 2.0 Button in Switcher Status Gadget', () => {
  const pagePath = path.resolve(import.meta.dirname, '../pages/QuotaDashboardPage.tsx')
  const pageSrc = fs.readFileSync(pagePath, 'utf8')

  it('verifies antigravity_logo.png asset exists in frontend assets and public', () => {
    const assetPath = path.resolve(import.meta.dirname, '../assets/antigravity_logo.png')
    const publicPath = path.resolve(import.meta.dirname, '../../public/antigravity_logo.png')
    assert.ok(fs.existsSync(assetPath), 'frontend/src/assets/antigravity_logo.png must exist')
    assert.ok(fs.existsSync(publicPath), 'frontend/public/antigravity_logo.png must exist')
    const stat = fs.statSync(assetPath)
    assert.ok(stat.size > 1000, 'antigravity_logo.png should be a valid non-empty image file')
  })

  it('imports antigravityLogo in QuotaDashboardPage', () => {
    assert.ok(
      pageSrc.includes("import antigravityLogo from '../assets/antigravity_logo.png'"),
      'QuotaDashboardPage must import antigravityLogo from assets'
    )
  })

  it('renders the logo button at the bottom-right corner of the Switcher Status gadget', () => {
    assert.ok(
      pageSrc.includes('id="btnLaunchAntigravity"'),
      'Action row must render btnLaunchAntigravity button'
    )
    assert.ok(
      pageSrc.includes("marginLeft: 'auto'"),
      'Logo button container must be pinned to the right corner using marginLeft auto'
    )
    assert.ok(
      pageSrc.includes("justifyContent: 'space-between'"),
      'Action row container must space between the left buttons and right logo button'
    )
  })

  it('ensures the button has no border or background (pure logo appearance)', () => {
    assert.ok(
      pageSrc.includes("background: 'none'"),
      'Button must have background: none'
    )
    assert.ok(
      pageSrc.includes("backgroundColor: 'transparent'"),
      'Button must have backgroundColor: transparent'
    )
    assert.ok(
      pageSrc.includes("border: 'none'"),
      'Button must have border: none'
    )
    assert.ok(
      pageSrc.includes("boxShadow: 'none'"),
      'Button must have boxShadow: none'
    )
  })

  it('matches the height of the two adjacent buttons exactly (32px) and specifies 32px square width', () => {
    // Both adjacent buttons and the logo button should share the 32px height
    assert.ok(
      pageSrc.includes("height: '32px'"),
      'Button must specify 32px height to match the two adjacent buttons'
    )
    assert.ok(
      pageSrc.includes("width: '32px'"),
      'Button must specify 32px width to maintain a square 4px-grid box'
    )
    // Check that the two buttons also have height: '32px'
    const scanMatch = pageSrc.includes("Scan Local Accounts")
    const addMatch = pageSrc.includes("Add Account")
    assert.ok(scanMatch && addMatch, 'Both adjacent buttons must exist')
  })

  it('renders a balanced 24px logo inside the 32px button box for visual balance', () => {
    assert.ok(
      pageSrc.includes("height: '24px'"),
      'Logo image height must be set to 24px'
    )
    assert.ok(
      pageSrc.includes("aspectRatio: '200 / 184'"),
      'Logo image must preserve 200/184 aspect ratio'
    )
  })

  it('displays "Launch Antigravity 2.0" text on hover via tooltip and title attribute', () => {
    assert.ok(
      pageSrc.includes('title="Launch Antigravity 2.0"'),
      'Button must have native title="Launch Antigravity 2.0"'
    )
    assert.ok(
      pageSrc.includes('Launch Antigravity 2.0'),
      'Tooltip popup must render Launch Antigravity 2.0 text'
    )
    assert.ok(
      pageSrc.includes('isLogoHovered'),
      'Must track hover state to reveal floating tooltip'
    )
  })

  it('provides a distinct hover effect so the user can easily tell it is an interactive button', () => {
    assert.ok(
      pageSrc.includes("isLogoHovered ? 'scale(1.12)' : 'scale(1)'"),
      'Button must scale up smoothly on hover'
    )
    assert.ok(
      pageSrc.includes("drop-shadow"),
      'Button must apply drop-shadow glow effect on hover'
    )
    assert.ok(
      pageSrc.includes("cursor: isLaunchingIDE ? 'wait' : 'pointer'"),
      'Button must display pointer cursor on hover'
    )
    assert.ok(
      pageSrc.includes("transition: 'transform 0.18s"),
      'Button must feature smooth CSS transition for hover effect'
    )
  })

  it('dispatches Antigravity launch when clicked with re-entrancy and error guards', () => {
    assert.ok(
      pageSrc.includes('handleLaunchAntigravity'),
      'Button must trigger handleLaunchAntigravity onClick'
    )
    assert.ok(
      pageSrc.includes('if (isLaunchingIDE) return'),
      'Must guard handleLaunchAntigravity against concurrent double-clicks'
    )
    assert.ok(
      pageSrc.includes('disabled={isLaunchingIDE}'),
      'Button must be disabled while launch request is in flight'
    )
    assert.ok(
      pageSrc.includes('api.launchHostIDE()') || pageSrc.includes('api.relaunchHostIDE()') || pageSrc.includes('electronAPI?.launchAntigravity'),
      'Must call launch Antigravity API'
    )
    assert.ok(
      pageSrc.includes('res.success === false'),
      'Must inspect launch result to surface backend errors instead of false positives'
    )
  })

  it('safeguards asynchronous timer callbacks against unmounted state updates', () => {
    assert.ok(
      pageSrc.includes('mountedRef.current'),
      'Must check mountedRef before setting feedback on unmounted component'
    )
  })

  it('supports keyboard navigation via focus and blur listeners', () => {
    assert.ok(
      pageSrc.includes('onFocus={() => setIsLogoHovered(true)}'),
      'Button must show tooltip on keyboard focus'
    )
    assert.ok(
      pageSrc.includes('onBlur={() => setIsLogoHovered(false)}'),
      'Button must hide tooltip on blur'
    )
  })

  it('guarantees bottom-right corner positioning even when action buttons wrap', () => {
    assert.ok(
      pageSrc.includes("alignItems: 'flex-end'"),
      'Action row must align items to flex-end so logo button stays at bottom-right corner'
    )
    assert.ok(
      pageSrc.includes("alignSelf: 'flex-end'"),
      'Logo button container must self-align to flex-end'
    )
  })

  it('provides tactile active/in-flight scale feedback and tooltip arrow', () => {
    assert.ok(
      pageSrc.includes("isLaunchingIDE ? 'scale(0.95)'"),
      'Must scale down on active/in-flight press for tactile feedback'
    )
    assert.ok(
      pageSrc.includes('borderTop:'),
      'Tooltip must render pointer arrow indicating target element'
    )
    assert.ok(
      pageSrc.includes("right: '11px'"),
      'Tooltip pointer arrow must be centered (right: 11px) over the 32px square button box'
    )
  })
})
