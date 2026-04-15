migrate((app) => {
  const adminOnly = "@request.auth.role = \"admin\""
  const species = app.findCollectionByNameOrId("species")

  if (!species) {
    throw new Error("species collection was not found.")
  }

  species.listRule = ""
  species.viewRule = ""
  species.createRule = adminOnly
  species.updateRule = adminOnly
  species.deleteRule = adminOnly

  app.save(species)
}, (app) => {
  try {
    const species = app.findCollectionByNameOrId("species")
    if (species) {
      const adminOnly = "@request.auth.role = \"admin\""
      species.listRule = adminOnly
      species.viewRule = adminOnly
      species.createRule = adminOnly
      species.updateRule = adminOnly
      species.deleteRule = adminOnly
      app.save(species)
    }
  } catch (_) {
    // Ignore rollback failures.
  }
})
