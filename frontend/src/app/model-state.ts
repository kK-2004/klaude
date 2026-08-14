import type { ModelCatalog } from '../lib/backend'

export const emptyModelCatalog: ModelCatalog = { activeId: '', profiles: [] }

export function hasConfiguredModel(model: string) {
  return model.trim().length > 0
}
