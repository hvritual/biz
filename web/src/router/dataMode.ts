export function routeContentEnabled(_previewMode: boolean, _surface: unknown): boolean {
  // A route that has not received a service adapter remains a complete,
  // tenant-isolated demo surface in local development. This is deliberate UI
  // selection, not an API-error fallback: API-backed routes still surface API
  // errors instead of substituting simulated success.
  return true
}
