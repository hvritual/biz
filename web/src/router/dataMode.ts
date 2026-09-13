export function routeContentEnabled(previewMode: boolean, surface: unknown): boolean {
  // These arguments remain part of the route-content contract even though the
  // current product decision enables complete demo surfaces for every route.
  void previewMode
  void surface

  // A route that has not received a service adapter remains a complete,
  // tenant-isolated demo surface in local development. This is deliberate UI
  // selection, not an API-error fallback: API-backed routes still surface API
  // errors instead of substituting simulated success.
  return true
}
