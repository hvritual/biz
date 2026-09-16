/** Checks design obligations only; this never renders or executes a page. */
export function validatePatternBindings(contract) {
  if (!contract.patterns) throw new Error('Page patterns must be declared in ui-contracts.json')
  for (const [name, pattern] of Object.entries(contract.patterns)) {
    if (pattern.required_regions.some((region) => pattern.optional_regions.includes(region))) {
      throw new Error(`${name}: a region cannot be both required and optional`)
    }
    for (const example of pattern.examples) {
      const page = contract.routes.find((route) => route.path === example)
      if (!page || page.template !== name) throw new Error(`${name}: example must use this pattern: ${example}`)
    }
  }
  for (const page of contract.routes) {
    const pattern = contract.patterns[page.template]
    if (!pattern || pattern.status !== 'implemented') throw new Error(`${page.path}: cannot consume a reserved/undefined pattern`)
    for (const region of pattern.required_regions) {
      if (page.required_regions.includes(region)) continue
      // Page-local exceptions still require reason, evidence and a review condition in the schema.
      const rule = `pattern.${page.template}.${region}`
      if (!page.exceptions?.some((exception) => exception.rule === rule)) {
        throw new Error(`${page.path}: missing mandatory pattern region ${region}`)
      }
    }
  }
}
