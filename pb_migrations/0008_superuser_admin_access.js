/// <reference path="../pb_data/types.d.ts" />
migrate((app) => {
  const adminOrSuperuser = "@request.auth.collectionName = \"_superusers\" || @request.auth.role = \"admin\""

  const users = app.findCollectionByNameOrId("users")
  const location = app.findCollectionByNameOrId("location")
  const species = app.findCollectionByNameOrId("species")

  if (!users || !location || !species) {
    throw new Error("Required collections (users/location/species) were not found.")
  }

  users.listRule = adminOrSuperuser
  users.viewRule = adminOrSuperuser
  users.createRule = "@request.context = \"oauth2\" || " + adminOrSuperuser
  users.updateRule = adminOrSuperuser
  users.deleteRule = adminOrSuperuser
  users.manageRule = adminOrSuperuser
  app.save(users)

  location.listRule = adminOrSuperuser
  location.viewRule = adminOrSuperuser
  location.createRule = adminOrSuperuser
  location.updateRule = adminOrSuperuser
  location.deleteRule = adminOrSuperuser
  app.save(location)

  species.listRule = adminOrSuperuser
  species.viewRule = adminOrSuperuser
  species.createRule = adminOrSuperuser
  species.updateRule = adminOrSuperuser
  species.deleteRule = adminOrSuperuser
  app.save(species)
}, (app) => {
  const adminOnly = "@request.auth.role = \"admin\""
  const oauthOrAdmin = "@request.context = \"oauth2\" || @request.auth.role = \"admin\""

  try {
    const users = app.findCollectionByNameOrId("users")
    if (users) {
      users.listRule = adminOnly
      users.viewRule = adminOnly
      users.createRule = oauthOrAdmin
      users.updateRule = adminOnly
      users.deleteRule = adminOnly
      users.manageRule = adminOnly
      app.save(users)
    }
  } catch (_) {
    // Ignore rollback failures.
  }

  try {
    const location = app.findCollectionByNameOrId("location")
    if (location) {
      location.listRule = ""
      location.viewRule = ""
      location.createRule = adminOnly
      location.updateRule = adminOnly
      location.deleteRule = adminOnly
      app.save(location)
    }
  } catch (_) {
    // Ignore rollback failures.
  }

  try {
    const species = app.findCollectionByNameOrId("species")
    if (species) {
      species.listRule = ""
      species.viewRule = ""
      species.createRule = adminOnly
      species.updateRule = adminOnly
      species.deleteRule = adminOnly
      app.save(species)
    }
  } catch (_) {
    // Ignore rollback failures.
  }
})
