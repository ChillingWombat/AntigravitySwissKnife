import { useReducer } from 'react'
import type { CustomModel } from '../../types'

type DialogMode = 'add' | 'edit'

export type ModelFormDialogState = {
  type: 'modelForm'
  mode: DialogMode
  initialModel: CustomModel | null
}

export type DeleteDialogState = {
  type: 'delete'
  model: { id: string; name: string }
  returnTo: ModelFormDialogState
}

export type DialogState =
  | { type: 'closed' }
  | ModelFormDialogState
  | DeleteDialogState
  | { type: 'gcpForm'; mode: DialogMode; initialModel: CustomModel | null }
  | { type: 'report' }

export type DialogAction =
  | { type: 'openModelForm'; mode: DialogMode; initialModel: CustomModel | null }
  | { type: 'openDelete'; model: { id: string; name: string } }
  | { type: 'openGcp'; mode: DialogMode; initialModel: CustomModel | null }
  | { type: 'openReport' }
  | { type: 'close' }
  | { type: 'backToModelForm' }

export const initialDialogState: DialogState = { type: 'closed' }

export const dialogStateReducer = (state: DialogState, action: DialogAction): DialogState => {
  switch (action.type) {
    case 'openModelForm':
      return {
        type: 'modelForm',
        mode: action.mode,
        initialModel: action.initialModel,
      }
    case 'openDelete':
      if (state.type !== 'modelForm') return state
      return {
        type: 'delete',
        model: action.model,
        returnTo: state,
      }
    case 'openGcp':
      return {
        type: 'gcpForm',
        mode: action.mode,
        initialModel: action.initialModel,
      }
    case 'openReport':
      return { type: 'report' }
    case 'close':
      return initialDialogState
    case 'backToModelForm':
      return state.type === 'delete' ? state.returnTo : state
  }
}

export const useDialogState = () => useReducer(dialogStateReducer, initialDialogState)
