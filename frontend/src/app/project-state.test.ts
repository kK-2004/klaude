import { describe, expect, it } from 'vitest'
import { removeProject } from './project-state'
import type { Project } from '../types/backend'

const project = (id: string): Project => ({
  id,
  name: id,
  rootPath: `/tmp/${id}`,
  pinned: false,
  createdAt: '',
  updatedAt: '',
})

describe('project state', () => {
  it('does not restore a project after a successful delete', () => {
    expect(removeProject([project('one'), project('two')], 'one')).toEqual([project('two')])
  })
})
