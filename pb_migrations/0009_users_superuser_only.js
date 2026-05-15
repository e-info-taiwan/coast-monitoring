/// <reference path="../pb_data/types.d.ts" />
migrate((app) => {
  const adminOrSuperuser =
    "@request.auth.collectionName = \"_superusers\" || @request.auth.role = \"admin\""
  const superuserOnly = "@request.auth.collectionName = \"_superusers\""

  const users = app.findCollectionByNameOrId("users")
  if (!users) {
    throw new Error("users collection not found")
  }

  // Reading users is still required for admin so observations can expand
  // the observer relation in API responses.
  users.listRule = adminOrSuperuser
  users.viewRule = adminOrSuperuser
  // OAuth2 callback still needs to be able to create user records.
  users.createRule = "@request.context = \"oauth2\" || " + superuserOnly
  // Mutations are superuser-only from now on.
  users.updateRule = superuserOnly
  users.deleteRule = superuserOnly
  users.manageRule = superuserOnly
  app.save(users)
}, (app) => {
  const adminOrSuperuser =
    "@request.auth.collectionName = \"_superusers\" || @request.auth.role = \"admin\""

  try {
    const users = app.findCollectionByNameOrId("users")
    if (users) {
      users.listRule = adminOrSuperuser
      users.viewRule = adminOrSuperuser
      users.createRule = "@request.context = \"oauth2\" || " + adminOrSuperuser
      users.updateRule = adminOrSuperuser
      users.deleteRule = adminOrSuperuser
      users.manageRule = adminOrSuperuser
      app.save(users)
    }
  } catch (_) {
    // Ignore rollback failures.
  }
})
