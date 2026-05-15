/// <reference path="../pb_data/types.d.ts" />
migrate((app) => {
  const observation = app.findCollectionByNameOrId("observation")
  if (!observation) {
    throw new Error("observation collection not found")
  }

  const adminOrSuperuser =
    "@request.auth.collectionName = \"_superusers\" || @request.auth.role = \"admin\""
  const adminOrSuperuserOrOwner =
    adminOrSuperuser + " || observer = @request.auth.id"
  const authed = "@request.auth.id != \"\""

  observation.listRule = adminOrSuperuserOrOwner
  observation.viewRule = adminOrSuperuserOrOwner
  observation.createRule = authed
  observation.updateRule = adminOrSuperuserOrOwner
  observation.deleteRule = adminOrSuperuserOrOwner
  app.save(observation)
}, (app) => {
  try {
    const observation = app.findCollectionByNameOrId("observation")
    if (observation) {
      const authed = "@request.auth.id != \"\""
      observation.listRule = authed
      observation.viewRule = authed
      observation.createRule = authed
      observation.updateRule = authed
      observation.deleteRule = authed
      app.save(observation)
    }
  } catch (_) {
    // Ignore rollback failures.
  }
})
