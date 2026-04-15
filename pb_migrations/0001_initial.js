migrate((app) => {
  const adminOnly = "@request.auth.role = \"admin\""
  const users = app.findCollectionByNameOrId("users")

  if (!users) {
    throw new Error("Default users auth collection was not found.")
  }

  users.listRule = adminOnly
  users.viewRule = adminOnly
  users.updateRule = adminOnly
  users.deleteRule = adminOnly
  users.manageRule = adminOnly
  users.createRule = ""
  users.authRule = ""
  users.passwordAuth.enabled = false
  users.oauth2.enabled = true
  users.oauth2.mappedFields = {
    id: "id",
    name: "name",
    username: "",
    avatarURL: "",
  }
  users.oauth2.providers = [
    {
      name: "google",
      displayName: "Google",
      clientId: $os.getenv("PB_GOOGLE_CLIENT_ID") || "",
      clientSecret: $os.getenv("PB_GOOGLE_CLIENT_SECRET") || "",
      authURL: "https://accounts.google.com/o/oauth2/v2/auth",
      tokenURL: "https://oauth2.googleapis.com/token",
      userInfoURL: "https://openidconnect.googleapis.com/v1/userinfo",
      pkce: true,
      extra: {
        scopes: ["openid", "email", "profile"],
      },
    },
  ]
  users.fields.add(
    new TextField({
      name: "name",
      required: true,
      max: 100,
      presentable: true,
    }),
    new SelectField({
      name: "role",
      required: true,
      maxSelect: 1,
      values: ["admin", "volunteer"],
      presentable: true,
    }),
    new RelationField({
      name: "createUser",
      collectionId: users.id,
      maxSelect: 1,
      required: false,
      presentable: true,
    }),
    new RelationField({
      name: "updateUser",
      collectionId: users.id,
      maxSelect: 1,
      required: false,
      presentable: true,
    }),
    new AutodateField({
      name: "createDate",
      onCreate: true,
      onUpdate: false,
      presentable: true,
    }),
    new AutodateField({
      name: "updateDate",
      onCreate: true,
      onUpdate: true,
      presentable: true,
    }),
  )
  app.save(users)

  const location = new Collection({ type: "base", name: "location" })
  location.listRule = adminOnly
  location.viewRule = adminOnly
  location.createRule = adminOnly
  location.updateRule = adminOnly
  location.deleteRule = adminOnly
  location.fields.add(
    new TextField({
      name: "chineseName",
      required: true,
      max: 100,
      presentable: true,
    }),
    new TextField({
      name: "englishName",
      required: true,
      max: 100,
      presentable: true,
    }),
    new RelationField({
      name: "createUser",
      collectionId: users.id,
      maxSelect: 1,
      required: false,
      presentable: true,
    }),
    new RelationField({
      name: "updateUser",
      collectionId: users.id,
      maxSelect: 1,
      required: false,
      presentable: true,
    }),
    new AutodateField({
      name: "createDate",
      onCreate: true,
      onUpdate: false,
      presentable: true,
    }),
    new AutodateField({
      name: "updateDate",
      onCreate: true,
      onUpdate: true,
      presentable: true,
    }),
  )
  app.save(location)

  const species = new Collection({ type: "base", name: "species" })
  species.listRule = adminOnly
  species.viewRule = adminOnly
  species.createRule = adminOnly
  species.updateRule = adminOnly
  species.deleteRule = adminOnly
  species.fields.add(
    new TextField({
      name: "chineseName",
      required: true,
      max: 100,
      presentable: true,
    }),
    new TextField({
      name: "englishName",
      required: true,
      max: 100,
      presentable: true,
    }),
    new RelationField({
      name: "createUser",
      collectionId: users.id,
      maxSelect: 1,
      required: false,
      presentable: true,
    }),
    new RelationField({
      name: "updateUser",
      collectionId: users.id,
      maxSelect: 1,
      required: false,
      presentable: true,
    }),
    new AutodateField({
      name: "createDate",
      onCreate: true,
      onUpdate: false,
      presentable: true,
    }),
    new AutodateField({
      name: "updateDate",
      onCreate: true,
      onUpdate: true,
      presentable: true,
    }),
  )
  app.save(species)
}, (app) => {
  const names = ["species", "location"]
  for (const name of names) {
    try {
      const collection = app.findCollectionByNameOrId(name)
      app.delete(collection)
    } catch (_) {
      // Ignore missing collections during rollback.
    }
  }
})
