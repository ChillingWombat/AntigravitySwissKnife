---
name: Antigravity Swiss Knife
description: A precise, local-first operator deck for Antigravity runtime control.
colors:
  accent: "#0b57d0"
  accent-hover: "#0842a0"
  accent-foreground: "#ffffff"
  accent-dark: "#a8c7fa"
  canvas: "#f8fafd"
  canvas-dark: "#131314"
  surface: "#ffffff"
  surface-dark: "#1e1f20"
  surface-variant: "#f1f3f4"
  border: "#e0e2e5"
  foreground: "#1f1f1f"
  foreground-muted: "#444746"
  selection: "#c2e7ff"
  success: "#137333"
  warning: "#b06000"
  danger: "#b3261e"
  code-background: "#f1f5f9"
typography:
  display:
    fontFamily: "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif"
    fontSize: "24px"
    fontWeight: 700
    lineHeight: 1.25
    letterSpacing: "-0.01em"
  headline:
    fontFamily: "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif"
    fontSize: "20px"
    fontWeight: 600
    lineHeight: 1.3
  title:
    fontFamily: "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif"
    fontSize: "16px"
    fontWeight: 600
    lineHeight: 1.4
  body:
    fontFamily: "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif"
    fontSize: "13px"
    fontWeight: 400
    lineHeight: 1.5
  label:
    fontFamily: "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif"
    fontSize: "12px"
    fontWeight: 600
    lineHeight: 1.3
    letterSpacing: "0.01em"
  mono:
    fontFamily: "'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace"
    fontSize: "12px"
    fontWeight: 400
    lineHeight: 1.5
rounded:
  sm: "4px"
  md: "6px"
  lg: "8px"
  pill: "9999px"
spacing:
  "2xs": "2px"
  xs: "4px"
  sm: "8px"
  md: "12px"
  lg: "16px"
  xl: "24px"
  "2xl": "32px"
components:
  button-primary:
    backgroundColor: "{colors.accent}"
    textColor: "{colors.accent-foreground}"
    typography: "{typography.label}"
    rounded: "{rounded.pill}"
    padding: "8px 16px"
    height: "34px"
  button-primary-hover:
    backgroundColor: "{colors.accent-hover}"
    textColor: "{colors.accent-foreground}"
    rounded: "{rounded.pill}"
  button-tonal:
    backgroundColor: "{colors.surface-variant}"
    textColor: "{colors.foreground}"
    typography: "{typography.label}"
    rounded: "{rounded.pill}"
    padding: "8px 16px"
    height: "34px"
  button-outlined:
    backgroundColor: transparent
    textColor: "{colors.foreground}"
    typography: "{typography.label}"
    rounded: "{rounded.pill}"
    padding: "7px 15px"
    height: "34px"
  card:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.foreground}"
    rounded: "{rounded.lg}"
    padding: "{spacing.xl}"
  input:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.foreground}"
    typography: "{typography.body}"
    rounded: "{rounded.lg}"
    padding: "8px 12px"
    height: "34px"
  nav-item:
    backgroundColor: transparent
    textColor: "{colors.foreground-muted}"
    typography: "{typography.label}"
    rounded: "{rounded.lg}"
    padding: "8px 12px"
    height: "36px"
  nav-item-active:
    backgroundColor: "{colors.surface-variant}"
    textColor: "{colors.foreground}"
    rounded: "{rounded.lg}"
  chip:
    backgroundColor: "{colors.surface-variant}"
    textColor: "{colors.foreground-muted}"
    typography: "{typography.label}"
    rounded: "{rounded.pill}"
    padding: "4px 8px"
    height: "22px"
  toggle-track-on:
    backgroundColor: "{colors.success}"
    rounded: "{rounded.pill}"
    width: "40px"
    height: "20px"
  toggle-track-off:
    backgroundColor: "{colors.border}"
    rounded: "{rounded.pill}"
    width: "40px"
    height: "20px"
  quota-gauge:
    backgroundColor: "{colors.surface}"
    rounded: "{rounded.lg}"
    size: "72px"
---

# Design System: Antigravity Swiss Knife

## Overview

**Creative North Star: "Precision Instrument Deck"**

The interface is an operator deck for local runtime control: compact, legible, and arranged around live state rather than presentation. Familiar desktop patterns carry the workflow. The visual identity comes from exact alignment, restrained blue signals, and the confidence of a tool that never hides whether it is connected, loading, or blocked.

This is an Operate-mode system. Density is deliberate, but visual noise is not. Every border, label, number, and control earns its place through grouping, state, or action. Surfaces stay quiet in both light and dark themes so quota, account, and connection changes remain the loudest information on screen.

**Key Characteristics:**

- Local-first operational clarity with visible runtime truth.
- One restrained blue accent for actions, selection, and focus.
- A 4px spacing grid with 1px tokenized borders.
- Fixed, compact type steps and tabular numerals for data.
- Flat surfaces at rest with low, functional elevation.
- Familiar controls with complete keyboard and state behavior.

## Colors

The palette is a cool, low-chroma instrument surface with one working accent and three explicit status colors.

### Primary

- **Precision Blue** (#0b57d0): Primary actions, active selection, focus rings, and links. Dark mode uses **Signal Blue** (#a8c7fa) with dark text.
- **Selection Wash** (#c2e7ff): Text selection against **Operator Ink** (#041e49) for immediate feedback without introducing a second accent.

### Secondary

- **Signal Green** (#137333): Healthy quota and enabled state.
- **Caution Amber** (#b06000): Warning and quota pressure.
- **Fault Red** (#b3261e): Errors, destructive actions, and exhausted quota.

### Neutral

- **Working Paper** (#f8fafd): Light application canvas.
- **Instrument White** (#ffffff): Cards, fields, and elevated surfaces.
- **Operator Ink** (#1f1f1f): Primary text.
- **Muted Graphite** (#444746): Secondary labels and inactive navigation.
- **Gauge Line** (#e0e2e5): Borders, dividers, and inactive switch tracks.
- **Night Canvas** (#131314) and **Night Surface** (#1e1f20): Dark-theme equivalents that preserve the same hierarchy.

**The One Accent Rule.** Primary blue marks actions, selected state, and focus. It never becomes page decoration.

## Typography

**Display Font:** Inter with the native system sans fallback  
**Body Font:** Inter with the same system fallback  
**Label/Mono Font:** JetBrains Mono for code and preformatted output only

**Character:** Inter keeps the deck neutral and sharp at compact sizes. Hierarchy comes from weight and fixed step changes, not spectacle. Monospace is reserved for code, logs, identifiers, and measurement contexts where character alignment carries meaning.

### Hierarchy

- **Display** (700, 24px, 1.25): Page and major workspace titles.
- **Headline** (600, 20px, 1.3): Primary card or section headings.
- **Title** (600, 16px, 1.4): Card titles and grouped controls.
- **Body** (400, 13px, 1.5): Controls, table content, and operational prose.
- **Label** (600, 12px, 1.3): Status labels, tabs, metadata, and compact actions.
- **Mono** (400, 12px, 1.5): Code, logs, IDs, and terminal output.

Percentage values and other changing figures use tabular numerals. Action labels do not wrap.

**The Stable Scale Rule.** UI type uses a fixed rem scale; viewport-driven type changes are not used.

## Layout

The desktop shell uses a 200px navigation rail, a 72px command header, and a flexible content well with 24px page padding. The base grid is 4px. Tight groups use 4-8px gaps, related controls use 12-16px, and major regions separate at 24-32px.

Information is grouped by containment and proximity. Cards use one level of boundary; nested cards are avoided. Tabs and filter chips wrap instead of clipping. Primary buttons and action labels stay on one line. Tables and dense telemetry can exceed prose measure; explanatory paragraphs remain within 65-75 characters.

The supported Electron viewport starts at 1216x684. Structural compression or drawer behavior must preserve the complete control set rather than shrinking labels or type.

## Elevation & Depth

The system is flat at rest. A 1px border and subtle tonal shift separate most surfaces. Small shadows provide low ambient depth, while larger shadows belong to overlays and explicitly elevated cards.

### Shadow Vocabulary

- **Ambient low** (`0 1px 2px 0 rgba(0, 0, 0, 0.05)`): Inputs, chips, and compact controls.
- **Panel lift** (`0 4px 6px -1px rgba(0, 0, 0, 0.08), 0 2px 4px -2px rgba(0, 0, 0, 0.04)`): Popovers and raised panels.
- **Floating layer** (`0 10px 25px -3px rgba(0, 0, 0, 0.08), 0 4px 10px -2px rgba(0, 0, 0, 0.04)`): Dialogs and genuinely detached surfaces.

**The Flat-at-Rest Rule.** Borders and tonal layers separate surfaces at rest. Shadows appear only for elevation or transient overlays.

## Shapes

Cards, panels, and grouped controls use 8px corners. Inputs and compact selection surfaces use 6px or 8px corners. Buttons use a 20px pill radius. Badges are fully rounded. Switches use a 9999px capsule with a circular thumb. Scrollbars use 4px rounding.

Borders are always 1px at rest and use the semantic border token. Focus rings are 2px with a 2px offset. Do not mix square and pill treatments inside the same control group.

## Components

Controls should feel familiar, compact, and exact. Every interactive component needs default, hover, focus, active, disabled, and unavailable behavior.

### Buttons

- **Shape:** Pill, 20px radius, 34px standard height.
- **Primary:** Precision Blue background, white text, 8px by 16px padding.
- **Tonal / Outlined:** Neutral surface with 1px border; hover shifts tone, not size.
- **Focus:** 2px Precision Blue outline with 2px offset.
- **Labels:** Single line, 12-13px, medium or semibold.

### Chips

- **Style:** Full-radius neutral surface, 4px by 8px padding, 11-12px semibold text.
- **State:** Selection changes background and text together; status chips pair color with a readable label.

### Cards / Containers

- **Corner Style:** 8px.
- **Background:** Instrument White in light mode, Night Surface in dark mode.
- **Border:** 1px semantic border.
- **Shadow Strategy:** Flat by default; use the elevation vocabulary only for overlays and raised state.
- **Internal Padding:** 16-24px, increasing with card scope.

### Inputs / Fields

- **Style:** 1px muted border, surface background, 8px radius, 8px by 12px padding.
- **Focus:** Primary border and visible ring; the caret uses the primary token.
- **Error / Disabled:** Error uses Fault Red plus text; disabled reduces emphasis without disappearing from layout.

### Navigation

- **Style:** Label left, icon in a consistent Lucide stroke, 8px radius, 36px row height.
- **Default / Hover / Active:** Muted text at rest; hover uses a neutral wash; active pairs tonal background with primary icon and semibold label.
- **Icons:** Lucide only, 16-20px, currentColor.

### Toggle Switch

The switch is a 40x20px capsule with a 16px thumb; the small variant is 32x16px with a 12px thumb. On uses Signal Green, off uses Gauge Line, and the thumb uses the semantic surface token. Keyboard focus draws a 2px primary ring around the track. The input always has an accessible label.

### Quota Displays

Circular gauges and horizontal bars use the same semantic health colors: green at 50% or above, amber below 50%, and red below 20%. Untracked values display `N/A` and omit active progress. Percentage text uses tabular numerals. Color is reinforced by the displayed value and title.

## Do's and Don'ts

### Do:

- **Do** use Lucide for every icon and keep one stroke vocabulary.
- **Do** keep action labels on one line and let tags wrap.
- **Do** show hover, focus, disabled, loading, empty, and error states.
- **Do** use tabular numerals for percentages, counters, and aligned telemetry.
- **Do** use semantic tokens in both themes instead of hardcoded surface colors.
- **Do** preserve familiar desktop control behavior before adding expression.

### Don't:

- **Don't** add decorative emojis, secondary icon sets, or unrequested gadget icons.
- **Don't** place explanatory subtitles under every heading or toggle.
- **Don't** use gradients, glass effects, glows, or large shadows as decoration.
- **Don't** use accent blue on inactive states or large passive surfaces.
- **Don't** show a success or connected state without live runtime evidence.
- **Don't** shrink type or clip controls to solve constrained layouts.
