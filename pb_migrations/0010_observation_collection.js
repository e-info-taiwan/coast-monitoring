/// <reference path="../pb_data/types.d.ts" />
migrate((app) => {
  const users = app.findCollectionByNameOrId("users")
  const location = app.findCollectionByNameOrId("location")
  const species = app.findCollectionByNameOrId("species")
  if (!users || !location || !species) {
    throw new Error("required collections (users/location/species) not found")
  }

  const authed = "@request.auth.id != \"\""

  // Use plain object-literal field definitions (same style as the event
  // collection migration) — the typed `new XxxField({...})` constructors
  // dropped all custom fields when applied via `new Collection(...)`.
  const observation = new Collection({
    type: "base",
    name: "observation",
    system: false,
    listRule: authed,
    viewRule: authed,
    createRule: authed,
    updateRule: authed,
    deleteRule: authed,
    fields: [
      {
        autogeneratePattern: "[a-z0-9]{15}",
        hidden: false,
        id: "text_obs_id",
        max: 15,
        min: 15,
        name: "id",
        pattern: "^[a-z0-9]+$",
        presentable: false,
        primaryKey: true,
        required: true,
        system: true,
        type: "text",
      },
      {
        hidden: false,
        id: "date_obs_date",
        max: "",
        min: "",
        name: "date",
        presentable: true,
        required: true,
        system: false,
        type: "date",
      },
      {
        cascadeDelete: false,
        collectionId: location.id,
        hidden: false,
        id: "relation_obs_location",
        maxSelect: 1,
        minSelect: 1,
        name: "location",
        presentable: true,
        required: true,
        system: false,
        type: "relation",
      },
      {
        cascadeDelete: false,
        collectionId: species.id,
        hidden: false,
        id: "relation_obs_species",
        maxSelect: 1,
        minSelect: 1,
        name: "species",
        presentable: true,
        required: true,
        system: false,
        type: "relation",
      },
      {
        hidden: false,
        id: "number_obs_count",
        max: null,
        min: 0,
        name: "count",
        onlyInt: true,
        presentable: true,
        required: true,
        system: false,
        type: "number",
      },
      {
        autogeneratePattern: "",
        hidden: false,
        id: "text_obs_notes",
        max: 1000,
        min: 0,
        name: "notes",
        pattern: "",
        presentable: false,
        primaryKey: false,
        required: false,
        system: false,
        type: "text",
      },
      {
        cascadeDelete: false,
        collectionId: users.id,
        hidden: false,
        id: "relation_obs_observer",
        maxSelect: 1,
        minSelect: 1,
        name: "observer",
        presentable: true,
        required: true,
        system: false,
        type: "relation",
      },
      {
        cascadeDelete: false,
        collectionId: users.id,
        hidden: false,
        id: "relation_obs_createUser",
        maxSelect: 1,
        minSelect: 0,
        name: "createUser",
        presentable: true,
        required: false,
        system: false,
        type: "relation",
      },
      {
        cascadeDelete: false,
        collectionId: users.id,
        hidden: false,
        id: "relation_obs_updateUser",
        maxSelect: 1,
        minSelect: 0,
        name: "updateUser",
        presentable: true,
        required: false,
        system: false,
        type: "relation",
      },
      {
        hidden: false,
        id: "autodate_obs_createDate",
        name: "createDate",
        onCreate: true,
        onUpdate: false,
        presentable: true,
        system: false,
        type: "autodate",
      },
      {
        hidden: false,
        id: "autodate_obs_updateDate",
        name: "updateDate",
        onCreate: true,
        onUpdate: true,
        presentable: true,
        system: false,
        type: "autodate",
      },
    ],
  })

  app.save(observation)
}, (app) => {
  try {
    const observation = app.findCollectionByNameOrId("observation")
    if (observation) {
      app.delete(observation)
    }
  } catch (_) {
    // Ignore rollback failures.
  }
})
