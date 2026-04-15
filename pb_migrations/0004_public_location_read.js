migrate((app) => {
  const adminOnly = "@request.auth.role = \"admin\""
  const location = app.findCollectionByNameOrId("location")

  if (!location) {
    throw new Error("location collection was not found.")
  }

  location.listRule = ""
  location.viewRule = ""
  location.createRule = adminOnly
  location.updateRule = adminOnly
  location.deleteRule = adminOnly

  app.save(location)
}, (app) => {
  try {
    const location = app.findCollectionByNameOrId("location")
    if (location) {
      const adminOnly = "@request.auth.role = \"admin\""
      location.listRule = adminOnly
      location.viewRule = adminOnly
      location.createRule = adminOnly
      location.updateRule = adminOnly
      location.deleteRule = adminOnly
      app.save(location)
    }
  } catch (_) {
    // Ignore rollback failures.
  }
})
