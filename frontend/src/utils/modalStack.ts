import { reactive } from 'vue'

export const openedModalStack = reactive<string[]>([])

export const pushModalToTop = (modalId: string) => {
  const existingIndex = openedModalStack.indexOf(modalId)
  if (existingIndex !== -1) {
    openedModalStack.splice(existingIndex, 1)
  }
  openedModalStack.push(modalId)
}

export const removeModalFromStack = (modalId: string) => {
  const existingIndex = openedModalStack.indexOf(modalId)
  if (existingIndex !== -1) {
    openedModalStack.splice(existingIndex, 1)
  }
}

export const isTopMostModal = (modalId: string) => openedModalStack[openedModalStack.length - 1] === modalId

export const getModalStackIndex = (modalId: string) => {
  const existingIndex = openedModalStack.indexOf(modalId)
  return existingIndex === -1 ? 0 : existingIndex
}
