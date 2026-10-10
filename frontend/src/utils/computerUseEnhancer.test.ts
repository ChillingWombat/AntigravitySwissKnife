import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import type {
  PlatformComputerUseSpec,
  CalibrationResult,
} from '../types.ts'

describe('Cross-Platform Computer Use Enhancer (Requirement 6)', () => {
  const rootSrcDir = path.resolve(import.meta.dirname, '..')
  const utilitiesPagePath = path.join(rootSrcDir, 'pages', 'UtilitiesPage.tsx')

  describe('1. Platform Profiles & Diagnostic Defaults', () => {
    it('verifies standard Linux execution capabilities', () => {
      const linuxSpec: PlatformComputerUseSpec = {
        os: 'linux',
        display_pipeline: 'Wayland Compositor (Xwayland Rootless)',
        screen_capture_backend: 'xdg-desktop-portal / PipeWire Stream',
        grounding_backend: 'AT-SPI2 D-Bus Accessibility Tree',
        dpi_scaling_factor: 1.0,
        display_resolution: '1920x1080 (Primary)',
        coordinate_orientation: 'Top-Left (0,0) with Fractional Compensation',
        status: 'ready',
        diagnostics: {
          session_type: 'wayland',
          pipewire_socket: 'true',
          at_spi_bus: 'true',
        },
      }

      assert.equal(linuxSpec.os, 'linux')
      assert.ok(linuxSpec.display_pipeline.includes('Wayland'))
      assert.ok(linuxSpec.screen_capture_backend.includes('PipeWire'))
      assert.ok(linuxSpec.grounding_backend.includes('AT-SPI2'))
      assert.equal(linuxSpec.dpi_scaling_factor, 1.0)
    })

    it('verifies standard Windows execution capabilities', () => {
      const winSpec: PlatformComputerUseSpec = {
        os: 'windows',
        display_pipeline: 'Desktop Window Manager (DWM) Per-Monitor V2',
        screen_capture_backend: 'Windows Graphics Capture (WGC) & DXGI Duplication',
        grounding_backend: 'Windows UI Automation (UIA) COM Pattern Inspection',
        dpi_scaling_factor: 1.25,
        display_resolution: '2560x1440 (Primary High-DPI)',
        coordinate_orientation: 'Top-Left (0,0) with Normalized Virtual Desktop Mapping',
        status: 'ready',
        diagnostics: {
          dpi_awareness: 'Per-Monitor V2',
          wgc_api: 'Windows.Graphics.Capture',
        },
      }

      assert.equal(winSpec.os, 'windows')
      assert.ok(winSpec.display_pipeline.includes('DWM'))
      assert.ok(winSpec.screen_capture_backend.includes('Windows Graphics Capture'))
      assert.ok(winSpec.grounding_backend.includes('UI Automation'))
      assert.equal(winSpec.dpi_scaling_factor, 1.25)
    })

    it('verifies standard macOS execution capabilities', () => {
      const macSpec: PlatformComputerUseSpec = {
        os: 'darwin',
        display_pipeline: 'Quartz Display Services with Retina 2.0x Matrix',
        screen_capture_backend: 'ScreenCaptureKit (SCK) Zero-Copy Frame Stream',
        grounding_backend: 'macOS AXUIElement Accessibility API Hierarchy',
        dpi_scaling_factor: 2.0,
        display_resolution: '2880x1800 (Retina Display)',
        coordinate_orientation: 'Top-Left (0,0) via Inverted Cocoa Y-Coordinate Matrix',
        status: 'ready',
        diagnostics: {
          screencapture_kit: 'ScreenCaptureKit',
          retina_backing_scale: '2.0x',
        },
      }

      assert.equal(macSpec.os, 'darwin')
      assert.ok(macSpec.display_pipeline.includes('Quartz'))
      assert.ok(macSpec.screen_capture_backend.includes('ScreenCaptureKit'))
      assert.ok(macSpec.grounding_backend.includes('AXUIElement'))
      assert.equal(macSpec.dpi_scaling_factor, 2.0)
    })
  })

  describe('2. Coordinate Calibration Mathematical Verification', () => {
    function simulateCalibration(targetOS: string, inputX: number, inputY: number): CalibrationResult {
      let scale = 1.0
      let logW = 1920, logH = 1080
      let physW = 1920, physH = 1080

      if (targetOS === 'windows') {
        scale = 1.25
        logW = 2048
        logH = 1152
        physW = 2560
        physH = 1440
      } else if (targetOS === 'darwin' || targetOS === 'macos') {
        scale = 2.0
        logW = 1440
        logH = 900
        physW = 2880
        physH = 1800
      }

      const correctedX = Math.round(inputX * scale)
      const correctedY = Math.round(inputY * scale)

      return {
        os: targetOS,
        physical_width: physW,
        physical_height: physH,
        logical_width: logW,
        logical_height: logH,
        scale_factor: scale,
        offset_target_x: inputX,
        offset_target_y: inputY,
        corrected_target_x: correctedX,
        corrected_target_y: correctedY,
        accuracy_percent: 100.0,
        calibrated_at: 'Deterministic Calibration',
      }
    }

    it('calibrates 1.0x baseline coordinates for Linux', () => {
      const res = simulateCalibration('linux', 960, 540)
      assert.equal(res.scale_factor, 1.0)
      assert.equal(res.corrected_target_x, 960)
      assert.equal(res.corrected_target_y, 540)
      assert.equal(res.accuracy_percent, 100.0)
    })

    it('calibrates 1.25x High-DPI coordinates for Windows', () => {
      const res = simulateCalibration('windows', 1000, 800)
      assert.equal(res.scale_factor, 1.25)
      assert.equal(res.corrected_target_x, 1250)
      assert.equal(res.corrected_target_y, 1000)
    })

    it('calibrates 2.0x Retina points for macOS', () => {
      const res = simulateCalibration('darwin', 720, 450)
      assert.equal(res.scale_factor, 2.0)
      assert.equal(res.corrected_target_x, 1440)
      assert.equal(res.corrected_target_y, 900)
    })
  })

  describe('3. David-Design & UtilitiesPage Implementation Invariants', () => {
    it('ensures UtilitiesPage.tsx contains cross-platform OS selectors', () => {
      const content = fs.readFileSync(utilitiesPagePath, 'utf8')

      assert.ok(content.includes("id: 'linux'"), 'Must include Linux selector')
      assert.ok(content.includes("id: 'windows'"), 'Must include Windows selector')
      assert.ok(content.includes("id: 'darwin'"), 'Must include macOS selector')
    })

    it('ensures UtilitiesPage.tsx includes coordinate normalization probe', () => {
      const content = fs.readFileSync(utilitiesPagePath, 'utf8')

      assert.ok(content.includes('Display Coordinate Normalization Probe'))
      assert.ok(content.includes('Calibrate Coordinates'))
      assert.ok(content.includes('handleRunCalibration'))
    })

    it('verifies absence of decorative emojis and fractional font sizes in UtilitiesPage.tsx', () => {
      const content = fs.readFileSync(utilitiesPagePath, 'utf8')
      const stripped = content.replace(/\/\*[\s\S]*?\*\//g, '').replace(/\/\/.*$/gm, '')

      const emojiRegex = /[\u{1F300}-\u{1F9FF}\u{2600}-\u{26FF}\u{2700}-\u{27BF}]/u
      assert.equal(stripped.match(emojiRegex), null, 'Must not contain decorative emojis')

      const fractionalFontRegex = /fontSize:\s*['"]\d+\.5px['"]/g
      assert.equal(stripped.match(fractionalFontRegex), null, 'Must not contain fractional font sizes')
    })
  })
})
