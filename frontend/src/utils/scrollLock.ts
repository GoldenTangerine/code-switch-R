type ScrollLockSnapshot = {
  bodyOverflow: string
  mainOverflow: string
}

let lockCount = 0
let snapshot: ScrollLockSnapshot | null = null

const getMainContent = () => document.querySelector('.main-content') as HTMLElement | null

export const lockScroll = () => {
  lockCount += 1
  if (lockCount !== 1) return

  const main = getMainContent()
  snapshot = {
    bodyOverflow: document.body.style.overflow,
    mainOverflow: main?.style.overflow ?? '',
  }

  document.body.style.overflow = 'hidden'
  if (main) main.style.overflow = 'hidden'
}

export const unlockScroll = () => {
  if (lockCount === 0) return

  lockCount -= 1
  if (lockCount !== 0) return

  const main = getMainContent()
  if (!snapshot) {
    document.body.style.overflow = ''
    if (main) main.style.overflow = ''
    return
  }

  document.body.style.overflow = snapshot.bodyOverflow
  if (main) main.style.overflow = snapshot.mainOverflow
  snapshot = null
}

