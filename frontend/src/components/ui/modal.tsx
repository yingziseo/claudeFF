import type { ReactNode } from 'react'
import * as Dialog from '@radix-ui/react-dialog'
import { X } from 'lucide-react'

export function Modal({ open, onClose, title, description, children, returnFocus, dismissible = true }: {
  open: boolean; onClose: () => void; title: string; description: string; children: ReactNode; returnFocus?: string; dismissible?: boolean
}) {
  return <Dialog.Root open={open} onOpenChange={(value) => { if (!value && dismissible) onClose() }}>
    <Dialog.Portal>
      <Dialog.Overlay className="modal-overlay" />
      <Dialog.Content className="modal" onEscapeKeyDown={(e) => { if(!dismissible) e.preventDefault() }} onInteractOutside={(e) => { if(!dismissible) e.preventDefault() }} onCloseAutoFocus={(event) => {
        if (returnFocus) { event.preventDefault(); document.getElementById(returnFocus)?.focus() }
      }}>
        <Dialog.Title className="modal-title">{title}</Dialog.Title>
        <Dialog.Description className="modal-description">{description}</Dialog.Description>
        {dismissible && <Dialog.Close className="icon-button modal-close" aria-label="关闭弹窗"><X size={18} /></Dialog.Close>}
        {children}
      </Dialog.Content>
    </Dialog.Portal>
  </Dialog.Root>
}
