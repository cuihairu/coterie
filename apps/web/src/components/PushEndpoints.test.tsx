// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { PushEndpoints } from './PushEndpoints'
import '@testing-library/jest-dom/vitest'

afterEach(cleanup)

const DEVICES = [
  { id: 's1', user_id: 'u1', endpoint: 'https://push.example.com/sub/1', created_at: '2026-10-13T00:00:00Z' },
  { id: 's2', user_id: 'u1', endpoint: 'https://other.example.com/x', created_at: '2026-10-12T00:00:00Z' },
]

describe('PushEndpoints', () => {
  it('renders nothing when there are no registered endpoints', () => {
    const { container } = render(<PushEndpoints devices={[]} onRevoke={() => {}} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('lists endpoint hosts with dates and tags this browser', () => {
    render(
      <PushEndpoints
        devices={DEVICES}
        thisEndpoint={DEVICES[0].endpoint}
        onRevoke={() => {}}
      />,
    )
    expect(screen.getByText('已登记推送端点')).toBeInTheDocument()
    expect(screen.getByText('push.example.com')).toBeInTheDocument()
    expect(screen.getByText('other.example.com')).toBeInTheDocument()
    expect(screen.getByText('此浏览器')).toBeInTheDocument()
    // Only the matching endpoint is tagged: the tag sits inside the
    // same <p> as its host, not next to the other row's.
    const tagged = screen.getByText('push.example.com').closest('p')
    expect(tagged).toHaveTextContent('此浏览器')
    expect(screen.getByText('other.example.com').closest('p')).not.toHaveTextContent('此浏览器')
  })

  it('falls back to the raw endpoint when the URL cannot parse', () => {
    render(
      <PushEndpoints
        devices={[{ ...DEVICES[0], endpoint: 'not a url' }]}
        onRevoke={() => {}}
      />,
    )
    expect(screen.getByText('not a url')).toBeInTheDocument()
  })

  it('passes the clicked row id to onRevoke and disables only that row', () => {
    const onRevoke = vi.fn()
    render(
      <PushEndpoints devices={DEVICES} busyId="s2" onRevoke={onRevoke} />,
    )
    fireEvent.click(screen.getAllByRole('button', { name: '注销' })[0])
    expect(onRevoke).toHaveBeenCalledWith('s1')
    expect(screen.getAllByRole('button', { name: '注销' })[1]).toBeDisabled()
    expect(screen.getAllByRole('button', { name: '注销' })[0]).toBeEnabled()
  })
})
