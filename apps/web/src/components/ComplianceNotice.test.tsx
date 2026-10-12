// @vitest-environment jsdom
import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { ComplianceNotice } from './ComplianceNotice'

afterEach(cleanup)

describe('ComplianceNotice', () => {
  // NFR-2 pins the prompt: the UI must tell the user to verify the
  // provider's terms, region limits, and sharing rights themselves.
  it('names terms, region, and sharing rights', () => {
    render(<ComplianceNotice />)
    const note = screen.getByRole('note')
    expect(note).toHaveTextContent('服务条款')
    expect(note).toHaveTextContent('地区限制')
    expect(note).toHaveTextContent('共享权限')
  })
})
