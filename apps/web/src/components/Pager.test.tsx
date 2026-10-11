// @vitest-environment jsdom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { Pager } from './Pager'

afterEach(cleanup)

describe('Pager', () => {
  it('renders nothing when one page covers the list', () => {
    const { container } = render(
      <Pager offset={0} pageSize={20} total={20} onOffsetChange={() => {}} />,
    )
    expect(container).toBeEmptyDOMElement()
  })

  it('disables the previous button on the first page', () => {
    render(<Pager offset={0} pageSize={20} total={45} onOffsetChange={() => {}} />)
    expect(screen.getByRole('button', { name: '上一页' })).toBeDisabled()
    expect(screen.getByRole('button', { name: '下一页' })).toBeEnabled()
  })

  it('disables the next button on the last page', () => {
    render(<Pager offset={40} pageSize={20} total={45} onOffsetChange={() => {}} />)
    expect(screen.getByRole('button', { name: '下一页' })).toBeDisabled()
    expect(screen.getByRole('button', { name: '上一页' })).toBeEnabled()
  })

  it('steps forward and back by the page size', () => {
    const onOffsetChange = vi.fn()
    const { rerender } = render(
      <Pager offset={0} pageSize={20} total={45} onOffsetChange={onOffsetChange} />,
    )
    fireEvent.click(screen.getByRole('button', { name: '下一页' }))
    expect(onOffsetChange).toHaveBeenCalledWith(20)

    rerender(<Pager offset={20} pageSize={20} total={45} onOffsetChange={onOffsetChange} />)
    fireEvent.click(screen.getByRole('button', { name: '上一页' }))
    expect(onOffsetChange).toHaveBeenCalledWith(0)
  })

  it('never goes below the first page', () => {
    const onOffsetChange = vi.fn()
    render(<Pager offset={20} pageSize={20} total={25} onOffsetChange={onOffsetChange} />)
    fireEvent.click(screen.getByRole('button', { name: '上一页' }))
    expect(onOffsetChange).toHaveBeenCalledWith(0)
  })
})
