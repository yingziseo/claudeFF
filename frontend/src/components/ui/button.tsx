import type { ButtonHTMLAttributes } from 'react'
import { Slot } from '@radix-ui/react-slot'
import { cva, type VariantProps } from 'class-variance-authority'
import { clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

const variants = cva('button', {
  variants: { variant: { primary: 'button-primary', secondary: 'button-secondary', ghost: 'button-ghost', danger: 'button-danger' } },
  defaultVariants: { variant: 'primary' },
})
type Props = ButtonHTMLAttributes<HTMLButtonElement> & VariantProps<typeof variants> & { asChild?: boolean }
export function Button({ variant, className, asChild, type = 'button', ...props }: Props) {
  const Component = asChild ? Slot : 'button'
  return <Component type={type} className={twMerge(clsx(variants({ variant }), className))} {...props} />
}
