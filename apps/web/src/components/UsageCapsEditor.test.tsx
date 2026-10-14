// @vitest-environment jsdom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { UsageCapsEditor } from './UsageCapsEditor'

afterEach(cleanup)

const caps = [
  { kind: 'usage' as const, label: '每成员每期用量上限', hint: '超出记录返回 409' },
  { kind: 'quota' as const, label: '全订阅每期用量上限', hint: '全体成员合计超出返回 409' },
]

describe('UsageCapsEditor', () => {
  it('shows unset placeholders for both caps', () => {
    render(<UsageCapsEditor caps={caps} isOwner onSave={vi.fn()} />)
    expect(screen.getByText('未设每成员每期用量上限。')).toBeInTheDocument()
    expect(screen.getByText('未设全订阅每期用量上限。')).toBeInTheDocument()
  })

  it('hides the controls from non-owners', () => {
    render(<UsageCapsEditor caps={caps} isOwner={false} onSave={vi.fn()} />)
    expect(screen.queryByText('设置上限')).not.toBeInTheDocument()
  })

  it('saves the edited cap under its own kind', async () => {
    const onSave = vi.fn().mockResolvedValue(undefined)
    render(<UsageCapsEditor caps={caps} isOwner onSave={onSave} />)
    fireEvent.click(screen.getAllByText('设置上限')[1]) // the quota row
    fireEvent.change(screen.getByPlaceholderText('单位，如 credits'), { target: { value: 'credits' } })
    fireEvent.change(screen.getByPlaceholderText('每期上限'), { target: { value: '100' } })
    fireEvent.click(screen.getByText('保存'))
    await waitFor(() =>
      expect(onSave).toHaveBeenCalledWith('quota', { unit: 'credits', per_period: '100' }),
    )
  })

  it('prefills the form when changing an existing cap', () => {
    render(
      <UsageCapsEditor
        caps={[{ ...caps[0], cap: { unit: 'GB', per_period: '50' } }, caps[1]]}
        isOwner
        onSave={vi.fn()}
      />,
    )
    expect(screen.getByText('50')).toBeInTheDocument()
    fireEvent.click(screen.getByText('修改'))
    expect(screen.getByPlaceholderText('单位，如 credits')).toHaveValue('GB')
    expect(screen.getByPlaceholderText('每期上限')).toHaveValue('50')
  })

  it('clears a cap with an explicit null save', async () => {
    const onSave = vi.fn().mockResolvedValue(undefined)
    render(
      <UsageCapsEditor
        caps={[{ ...caps[0], cap: { unit: 'GB', per_period: '50' } }, caps[1]]}
        isOwner
        onSave={onSave}
      />,
    )
    fireEvent.click(screen.getByText('修改'))
    fireEvent.click(screen.getByText('清除上限'))
    await waitFor(() => expect(onSave).toHaveBeenCalledWith('usage', null))
  })

  it('requires both fields before saving', () => {
    const onSave = vi.fn()
    render(<UsageCapsEditor caps={caps} isOwner onSave={onSave} />)
    fireEvent.click(screen.getAllByText('设置上限')[0])
    fireEvent.click(screen.getByText('保存'))
    expect(screen.getByText('单位与上限均必填')).toBeInTheDocument()
    expect(onSave).not.toHaveBeenCalled()
  })

  it('surfaces save failures inline', async () => {
    const onSave = vi.fn().mockRejectedValue(new Error('boom'))
    render(<UsageCapsEditor caps={caps} isOwner onSave={onSave} />)
    fireEvent.click(screen.getAllByText('设置上限')[0])
    fireEvent.change(screen.getByPlaceholderText('单位，如 credits'), { target: { value: 'credits' } })
    fireEvent.change(screen.getByPlaceholderText('每期上限'), { target: { value: '100' } })
    fireEvent.click(screen.getByText('保存'))
    await waitFor(() => expect(screen.getByText('boom')).toBeInTheDocument())
  })
})
