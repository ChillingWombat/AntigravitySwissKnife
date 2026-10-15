/**
 * ToggleSwitch.test.ts — Component DOM Behavior Tests for ToggleSwitch
 *
 * Demonstrates genuine React component mounting into a simulated DOM environment,
 * testing element hierarchy, role attributes, visual state transitions,
 * accessibility attributes, and real user click interaction handling.
 */

import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import React from 'react'
import { mount } from '../../test-dom'
import { ToggleSwitch } from '../ToggleSwitch'

describe('Component DOM: ToggleSwitch (<ToggleSwitch />)', () => {
  it('mounts in unchecked state and reflects aria-checked="false"', async () => {
    let changedTo: boolean | null = null
    const { querySelector, unmount } = await mount(
      React.createElement(ToggleSwitch, {
        checked: false,
        onChange: (val: boolean) => {
          changedTo = val
        },
        id: 'test-toggle-1',
        ariaLabel: 'Enable Feature',
      })
    )

    const input = querySelector<HTMLInputElement>('input[role="switch"]')
    assert.ok(input, 'Must render input element with role="switch"')
    assert.equal(input.checked, false, 'Checkbox input must be unchecked')
    assert.equal(input.getAttribute('aria-checked'), 'false', 'aria-checked must be "false"')
    assert.equal(input.id, 'test-toggle-1', 'Must forward id attribute')
    assert.equal(input.getAttribute('aria-label'), 'Enable Feature', 'Must forward aria-label')

    // Track span background color for off state
    const trackSpan = querySelector<HTMLSpanElement>('label > span')
    assert.ok(trackSpan, 'Track span must be present')
    assert.equal(trackSpan.style.backgroundColor, '#cbd5e1', 'Off state background must be #cbd5e1')
    assert.equal(changedTo, null, 'onChange must not be called upon initial mount')

    unmount()
  })

  it('handles user click events and fires onChange with toggled boolean value', async () => {
    const recordedEvents: boolean[] = []
    const { querySelector, click, unmount } = await mount(
      React.createElement(ToggleSwitch, {
        checked: false,
        onChange: (val: boolean) => {
          recordedEvents.push(val)
        },
      })
    )

    const label = querySelector<HTMLLabelElement>('label')
    assert.ok(label, 'Must render enclosing label element')

    await click(label)
    assert.ok(recordedEvents.length >= 1, 'onChange callback must be triggered on click')
    assert.equal(recordedEvents[0], true, 'Clicking unchecked switch must invoke onChange(true)')

    unmount()
  })

  it('mounts in checked state with active green track and translated knob', async () => {
    const { querySelector, unmount } = await mount(
      React.createElement(ToggleSwitch, {
        checked: true,
        onChange: () => {},
        size: 'md',
      })
    )

    const input = querySelector<HTMLInputElement>('input[role="switch"]')
    assert.ok(input)
    assert.equal(input.checked, true, 'Checkbox input must be checked')
    assert.equal(input.getAttribute('aria-checked'), 'true', 'aria-checked must be "true"')

    const trackSpan = querySelector<HTMLSpanElement>('label > span')
    assert.ok(trackSpan)
    assert.equal(trackSpan.style.backgroundColor, '#16a34a', 'Active state background must be #16a34a (green)')

    const knob = querySelector<HTMLSpanElement>('label > span > span')
    assert.ok(knob, 'Knob span element must be present')
    assert.equal(knob.style.transform, 'translateX(20px)', 'Knob must translate 20px in md size when checked')

    unmount()
  })

  it('prevents click interactions and enforces disabled visual styles when disabled', async () => {
    let wasClicked = false
    const { querySelector, click, unmount } = await mount(
      React.createElement(ToggleSwitch, {
        checked: false,
        disabled: true,
        onChange: () => {
          wasClicked = true
        },
      })
    )

    const label = querySelector<HTMLLabelElement>('label')
    assert.ok(label)
    assert.equal(label.style.cursor, 'not-allowed', 'Disabled toggle must display not-allowed cursor')

    const input = querySelector<HTMLInputElement>('input[role="switch"]')
    assert.ok(input)
    assert.equal(input.disabled, true, 'Underlying input must have disabled attribute')

    const track = querySelector<HTMLSpanElement>('label > span')
    assert.ok(track)
    assert.equal(track.style.opacity, '0.6', 'Disabled track must render with 0.6 opacity')

    await click(label)
    assert.equal(wasClicked, false, 'Clicking disabled toggle must NOT trigger onChange')

    unmount()
  })

  it('supports size="sm" with smaller knob and translation', async () => {
    const { querySelector, unmount } = await mount(
      React.createElement(ToggleSwitch, {
        checked: true,
        onChange: () => {},
        size: 'sm',
      })
    )

    const track = querySelector<HTMLSpanElement>('label > span')
    assert.ok(track)
    assert.equal(track.style.width, '32px', 'sm track width must be 32px')
    assert.equal(track.style.height, '16px', 'sm track height must be 16px')

    const knob = querySelector<HTMLSpanElement>('label > span > span')
    assert.ok(knob)
    assert.equal(knob.style.width, '12px', 'sm knob width must be 12px')
    assert.equal(knob.style.transform, 'translateX(16px)', 'sm knob translate must be 16px when checked')

    unmount()
  })
})
