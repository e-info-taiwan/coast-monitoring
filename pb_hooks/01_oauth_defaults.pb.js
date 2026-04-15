onRecordAuthWithOAuth2Request((e) => {
  if (!e.collection || e.collection.name !== "users" || !e.isNewRecord) {
    return
  }

  const bootstrapAdminEmail = (($os.getenv("PB_BOOTSTRAP_ADMIN_EMAIL") || "") + "").trim().toLowerCase()
  const bootstrapAdminName = (($os.getenv("PB_BOOTSTRAP_ADMIN_NAME") || "") + "").trim()
  const currentEmail = (
    (e.oAuth2User && e.oAuth2User.email) ||
    e.createData.email ||
    ""
  )
    .trim()
    .toLowerCase()

  const displayName = (e.oAuth2User && e.oAuth2User.name ? e.oAuth2User.name : "").trim()
  const fallbackName =
    displayName ||
    (e.oAuth2User && e.oAuth2User.email ? e.oAuth2User.email.split("@")[0] : "") ||
    "Volunteer"

  e.createData.role = "volunteer"
  e.createData.name = e.createData.name || fallbackName

  let hasAdmin = false
  try {
    e.app.findFirstRecordByFilter("users", "role = 'admin'")
    hasAdmin = true
  } catch (_) {
    hasAdmin = false
  }

  if (!hasAdmin && bootstrapAdminEmail && currentEmail === bootstrapAdminEmail) {
    e.createData.role = "admin"
    if (bootstrapAdminName) {
      e.createData.name = bootstrapAdminName
    }
  }
})
