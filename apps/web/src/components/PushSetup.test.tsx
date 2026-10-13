// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { PushSetup } from './PushSetup'
import '@testing-library/jest-dom/vitest'

afterEach(cleanup)

describe('PushSetup', () => {
  it('renders nothing when the browser cannot subscribe', () => {
    const { container } = render(
      <PushSetup supported={false} enabled={false} onEnable={() => {}} onDisable={() => {}} />,
    )
    expect(container).toBeEmptyDOMElement()
  })

  it('offers enabling when push is off', () => {
    const onEnable = vi.fn()
    render(
      <PushSetup supported enabled={false} onEnable={onEnable} onDisable={() => {}} />,
    )
    expect(screen.getByText('浏览器推送')).toBeInTheDocument()
    expect(screen.getByText(/开启后通知会直接送达此浏览器/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '开启推送' }))
    expect(onEnable).toHaveBeenCalledTimes(1)
  })

  it('offers disabling when push is on', () => {
    const onDisable = vi.fn()
    render(
      <PushSetup supported enabled busy onEnable={() => {}} onDisable={onDisable} />,
    )
    const button = screen.getByRole('button', { name: '关闭推送' })
    expect(screen.getByText(/此浏览器会收到系统级通知/)).toBeInTheDocument()
    expect(button).toBeDisabled()
    fireEvent.click(button)
    expect(onDisable).not.toHaveBeenCalled()
  })

  it('shows a failure message from the flow', () => {
    render(
      <PushSetup
        supported
        enabled={false}
        message="通知权限被拒绝"
        onEnable={() => {}}
        onDisable={() => {}}
      />,
    )
    expect(screen.getByRole('alert')).toHaveTextContent('通知权限被拒绝')
  })
})
