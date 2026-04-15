migrate((app) => {
  const adminOnly = "@request.auth.role = \"admin\""
  const users = app.findCollectionByNameOrId("users")

  if (!users) {
    throw new Error("Default users auth collection was not found.")
  }

  const devPasswordAuth = String($os.getenv("PB_DEV_PASSWORD_AUTH") || "").trim().toLowerCase() === "true"
  const googleClientId = String($os.getenv("PB_GOOGLE_CLIENT_ID") || "").trim()
  const googleClientSecret = String($os.getenv("PB_GOOGLE_CLIENT_SECRET") || "").trim()

  users.listRule = adminOnly
  users.viewRule = adminOnly
  users.updateRule = adminOnly
  users.deleteRule = adminOnly
  users.manageRule = adminOnly
  users.authRule = ""
  users.createRule = ""
  users.passwordAuth.enabled = devPasswordAuth
  users.oauth2.enabled = !devPasswordAuth && Boolean(googleClientId && googleClientSecret)

  if (users.oauth2.enabled) {
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
        clientId: googleClientId,
        clientSecret: googleClientSecret,
        authURL: "https://accounts.google.com/o/oauth2/v2/auth",
        tokenURL: "https://oauth2.googleapis.com/token",
        userInfoURL: "https://openidconnect.googleapis.com/v1/userinfo",
        pkce: true,
        extra: {
          scopes: ["openid", "email", "profile"],
        },
      },
    ]
  } else {
    users.oauth2.providers = []
  }

  app.save(users)

}, (app) => {
  try {
    const users = app.findCollectionByNameOrId("users")
    if (users) {
      users.passwordAuth.enabled = false
      users.oauth2.enabled = true
      app.save(users)
    }
  } catch (_) {
    // Ignore rollback failures.
  }
})
