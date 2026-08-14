import type { Project } from '../types/backend'

export function removeProject(projects: Project[], projectID: string): Project[] {
  return projects.filter((project) => project.id !== projectID)
}
