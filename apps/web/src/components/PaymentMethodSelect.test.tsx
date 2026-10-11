// @vitest-environment jsdom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { chargeMethodFor, PaymentMethodSelect } from './PaymentMethodSelect'

afterEach(cleanup)

describe('PaymentMethodSelect', () => {
  it('stays hidden while only one channel is registered', () => {
    const { container } = render(
      <PaymentMethodSelect methods={['manual']} value="manual" onChange={() => {}} />,
    )
    expect(container).toBeEmptyDOMElement()
  })

  it('lists every channel once there is a choice', () => {
    render(
      <PaymentMethodSelect
        methods={['manual', 'sandbox']}
        value="manual"
        onChange={() => {}}
      />,
    )
    const select = screen.getByRole('combobox')
    expect(select).toHaveValue('manual')
    expect(screen.getByRole('option', { name: 'sandbox' })).toBeInTheDocument()
  })

  it('reports the picked channel', () => {
    const onChange = vi.fn()
    render(
      <PaymentMethodSelect
        methods={['manual', 'sandbox', 'stripe']}
        value="manual"
        onChange={onChange}
      />,
    )
    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'stripe' } })
    expect(onChange).toHaveBeenCalledWith('stripe')
  })
})

describe('chargeMethodFor', () => {
  it('prefers the user pick, then the first channel, then manual', () => {
    expect(chargeMethodFor(['manual', 'sandbox'], 'sandbox')).toBe('sandbox')
    expect(chargeMethodFor(['manual', 'sandbox'], undefined)).toBe('manual')
    expect(chargeMethodFor(['sandbox'], 'sandbox')).toBe('sandbox')
    expect(chargeMethodFor([], undefined)).toBe('manual')
  })
})
