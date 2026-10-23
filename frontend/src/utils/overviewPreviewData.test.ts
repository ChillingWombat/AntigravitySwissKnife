import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import * as overviewModule from './overviewPreviewData.ts'
import {
  OVERVIEW_PREVIEW_HEADER,
  OVERVIEW_DIVISION_BADGE_LABELS,
} from './overviewPreviewData.ts'

describe('Overview Panel Preview Types & Clean Header Contracts', () => {
  it('enforces clean, shortened header wording without throat-clearing prefix', () => {
    assert.equal(OVERVIEW_PREVIEW_HEADER, 'Overview Panel Preview')
    assert.ok(
      !OVERVIEW_PREVIEW_HEADER.includes('Live Preview:'),
      'Header must not contain redundant "Live Preview:" prefix'
    )
    assert.ok(
      !OVERVIEW_PREVIEW_HEADER.includes('Antigravity'),
      'Header must be concise and avoid redundant app name repetition'
    )
  })

  it('shortens division style badge label and eliminates parenthetical commentary', () => {
    assert.equal(OVERVIEW_DIVISION_BADGE_LABELS.divider_line, 'Divider Lines')
    assert.equal(OVERVIEW_DIVISION_BADGE_LABELS.border_zone, 'Border Zones')

    assert.ok(
      !OVERVIEW_DIVISION_BADGE_LABELS.border_zone.includes('('),
      'Badge label must not contain opening parenthesis'
    )
    assert.ok(
      !OVERVIEW_DIVISION_BADGE_LABELS.border_zone.includes(')'),
      'Badge label must not contain closing parenthesis'
    )
    assert.ok(
      !OVERVIEW_DIVISION_BADGE_LABELS.border_zone.includes('Whiter BG'),
      'Badge label must eliminate "(Whiter BG)" commentary'
    )
  })

  it('verifies complete elimination of synthetic mock constants and DEMO_PROJECT_NAME facade', () => {
    assert.equal((overviewModule as any).DEMO_PROJECT_NAME, undefined, 'DEMO_PROJECT_NAME must be eliminated')
    assert.equal((overviewModule as any).SYNTHETIC_SUBAGENTS_COUNT, undefined, 'SYNTHETIC_SUBAGENTS_COUNT must be eliminated')
    assert.equal((overviewModule as any).SYNTHETIC_FILES_COUNT, undefined, 'SYNTHETIC_FILES_COUNT must be eliminated')
    assert.equal((overviewModule as any).SYNTHETIC_UPLOADS_COUNT, undefined, 'SYNTHETIC_UPLOADS_COUNT must be eliminated')
    assert.equal((overviewModule as any).SYNTHETIC_OVERVIEW_FIXTURES, undefined, 'SYNTHETIC_OVERVIEW_FIXTURES must be eliminated')
  })
})
