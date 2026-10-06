import React from 'react'
import { COMPONENT_TOKENS } from '../utils/layoutTokens'

export interface ToggleSwitchProps {
  checked: boolean
  onChange: (checked: boolean) => void
  disabled?: boolean
  size?: 'sm' | 'md'
  id?: string
  name?: string
  ariaLabel?: string
  style?: React.CSSProperties
}

export const ToggleSwitch: React.FC<ToggleSwitchProps> = ({
  checked,
  onChange,
  disabled = false,
  size = 'md',
  id,
  name,
  ariaLabel,
  style,
}) => {
  const isSm = size === 'sm'
  const width = isSm ? COMPONENT_TOKENS.TOGGLE_SM_WIDTH : COMPONENT_TOKENS.TOGGLE_MD_WIDTH
  const height = isSm ? COMPONENT_TOKENS.TOGGLE_SM_HEIGHT : COMPONENT_TOKENS.TOGGLE_MD_HEIGHT
  const knobSize = isSm ? COMPONENT_TOKENS.TOGGLE_SM_KNOB : COMPONENT_TOKENS.TOGGLE_MD_KNOB
  const translate = width - knobSize - 4 // md: 36 - 16 - 4 = 16px; sm: 28 - 12 - 4 = 12px

  return (
    <label
      onClick={(e) => {
        if (!disabled) {
          e.preventDefault()
          onChange(!checked)
        }
      }}
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        cursor: disabled ? 'not-allowed' : 'pointer',
        userSelect: 'none',
        verticalAlign: 'middle',
        position: 'relative',
        ...style,
      }}
    >
      <input
        type="checkbox"
        id={id}
        name={name}
        role="switch"
        aria-checked={checked}
        aria-label={ariaLabel}
        checked={checked}
        disabled={disabled}
        onChange={(e) => onChange(e.target.checked)}
        style={{
          position: 'absolute',
          opacity: 0,
          width: 0,
          height: 0,
          margin: 0,
          pointerEvents: 'none',
        }}
      />
      <span
        style={{
          width: `${width}px`,
          height: `${height}px`,
          padding: '2px',
          borderRadius: '9999px',
          backgroundColor: checked ? '#16a34a' : '#cbd5e1',
          display: 'inline-flex',
          alignItems: 'center',
          transition: 'background-color 0.2s cubic-bezier(0.4, 0, 0.2, 1)',
          boxSizing: 'border-box',
          flexShrink: 0,
          opacity: disabled ? 0.6 : 1,
          boxShadow: 'inset 0 1px 2px rgba(0, 0, 0, 0.05)',
        }}
      >
        <span
          style={{
            width: `${knobSize}px`,
            height: `${knobSize}px`,
            borderRadius: '50%',
            backgroundColor: '#ffffff',
            boxShadow: '0 1px 3px rgba(0, 0, 0, 0.25)',
            transform: `translateX(${checked ? translate : 0}px)`,
            transition: 'transform 0.2s cubic-bezier(0.4, 0, 0.2, 1)',
            display: 'block',
          }}
        />
      </span>
    </label>
  )
}
