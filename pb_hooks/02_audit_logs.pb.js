function isAuthNoiseRequest(info) {
  const body = info?.body || {}
  return Boolean(
    body.identity ||
      body.password ||
      body.provider ||
      body.code ||
      body.codeVerifier ||
      body.redirectURL ||
      body.otp ||
      body.refreshToken,
  )
}


onRecordCreateRequest((event) => {
  if (!event.collection || !["users", "location", "species", "observation"].includes(event.collection.name)) {
    return event.next()
  }

  const info = (() => {
    try {
      return typeof event.requestInfo === "function" ? event.requestInfo() : null
    } catch (_) {
      return null
    }
  })()
  const auth = (info && info.auth) || event.auth || null
  const actorCollectionName =
    (auth && typeof auth.collection === "function" && auth.collection() && auth.collection().name) ||
    (auth && typeof auth.isSuperuser === "function" && auth.isSuperuser() ? "_superusers" : "")
  const actorLabel = (() => {
    if (!auth) {
      return "anonymous"
    }
    try {
      if (typeof auth.get === "function") {
        return auth.get("name") || auth.get("email") || auth.id || "anonymous"
      }
    } catch (_) {
      // Fall through.
    }
    return auth.name || auth.email || auth.id || "anonymous"
  })()
  const actorMeta = (() => {
    if (actorCollectionName === "users") {
      return {
        actorType: "user",
        actorUser: auth.id || "",
        actorLabel,
      }
    }

    if (actorCollectionName === "_superusers") {
      return {
        actorType: "superuser",
        actorUser: "",
        actorLabel,
      }
    }

    return {
      actorType: "anonymous",
      actorUser: "",
      actorLabel,
    }
  })()
  const userAgent = info?.headers?.["user-agent"] || info?.headers?.["User-Agent"] || ""
  const result = event.next()
  const auditLogs = event.app.findCollectionByNameOrId("audit_logs")

  if (auditLogs) {
    const after = event.record
      ? {
          id: event.record.id || "",
          targetCollection:
            (typeof event.record.collection === "function" && event.record.collection() && event.record.collection().name) ||
            event.collection.name ||
            "",
          fields: (() => {
            try {
              const raw = typeof event.record.fieldsData === "function" ? event.record.fieldsData() : {}
              return raw === undefined ? undefined : JSON.parse(JSON.stringify(raw))
            } catch (_) {
              return typeof event.record.fieldsData === "function" ? event.record.fieldsData() : {}
            }
          })(),
        }
      : null

    const record = new Record(auditLogs, {
      action: "create",
      targetCollection: event.collection.name,
      targetRecordId: event.record?.id || "",
      actorType: actorMeta.actorType,
      actorUser: actorMeta.actorUser,
      actorLabel: actorMeta.actorLabel,
      method: info?.method || "",
      context: info?.context || "",
      ip: typeof event.realIP === "function" ? event.realIP() : "",
      userAgent,
      before: null,
      after,
    })

    try {
      event.app.save(record)
    } catch (error) {
      console.error("audit_logs create failed", error)
    }
  }

  return result
})

onRecordUpdateRequest((event) => {
  if (!event.collection || !["users", "location", "species", "observation"].includes(event.collection.name)) {
    return event.next()
  }

  const beforeRecord = event.record
  const beforeSnapshot = beforeRecord
    ? {
        id: beforeRecord.id || "",
        targetCollection:
          (typeof beforeRecord.collection === "function" && beforeRecord.collection() && beforeRecord.collection().name) ||
          event.collection.name ||
          "",
        fields: (() => {
          try {
            const raw = typeof beforeRecord.fieldsData === "function" ? beforeRecord.fieldsData() : {}
            return raw === undefined ? undefined : JSON.parse(JSON.stringify(raw))
          } catch (_) {
            return typeof beforeRecord.fieldsData === "function" ? beforeRecord.fieldsData() : {}
          }
        })(),
      }
    : null

  const info = (() => {
    try {
      return typeof event.requestInfo === "function" ? event.requestInfo() : null
    } catch (_) {
      return null
    }
  })()
  const isAuthNoise =
    Boolean(
      info?.body?.identity ||
        info?.body?.password ||
        info?.body?.provider ||
        info?.body?.code ||
        info?.body?.codeVerifier ||
        info?.body?.redirectURL ||
        info?.body?.otp ||
        info?.body?.refreshToken,
    )
  if (event.collection.name === "users" && isAuthNoise) {
    return event.next()
  }
  const auth = (info && info.auth) || event.auth || null
  const actorCollectionName =
    (auth && typeof auth.collection === "function" && auth.collection() && auth.collection().name) ||
    (auth && typeof auth.isSuperuser === "function" && auth.isSuperuser() ? "_superusers" : "")
  const actorLabel = (() => {
    if (!auth) {
      return "anonymous"
    }
    try {
      if (typeof auth.get === "function") {
        return auth.get("name") || auth.get("email") || auth.id || "anonymous"
      }
    } catch (_) {
      // Fall through.
    }
    return auth.name || auth.email || auth.id || "anonymous"
  })()
  const actorMeta = (() => {
    if (actorCollectionName === "users") {
      return {
        actorType: "user",
        actorUser: auth.id || "",
        actorLabel,
      }
    }

    if (actorCollectionName === "_superusers") {
      return {
        actorType: "superuser",
        actorUser: "",
        actorLabel,
      }
    }

    return {
      actorType: "anonymous",
      actorUser: "",
      actorLabel,
    }
  })()
  const userAgent = info?.headers?.["user-agent"] || info?.headers?.["User-Agent"] || ""
  const result = event.next()
  const auditLogs = event.app.findCollectionByNameOrId("audit_logs")

  if (auditLogs) {
    const after = event.record
      ? {
          id: event.record.id || "",
          targetCollection:
            (typeof event.record.collection === "function" && event.record.collection() && event.record.collection().name) ||
            event.collection.name ||
            "",
          fields: (() => {
            try {
              const raw = typeof event.record.fieldsData === "function" ? event.record.fieldsData() : {}
              return raw === undefined ? undefined : JSON.parse(JSON.stringify(raw))
            } catch (_) {
              return typeof event.record.fieldsData === "function" ? event.record.fieldsData() : {}
            }
          })(),
        }
      : null

    const ignored = new Set(["id", "createDate", "updateDate"])
    const changed = (() => {
      if (!beforeSnapshot || !after) {
        return true
      }
      const beforeFields = beforeSnapshot.fields || {}
      const afterFields = after.fields || {}
      const keys = new Set([...Object.keys(beforeFields), ...Object.keys(afterFields)])
      for (const key of keys) {
        if (ignored.has(key)) {
          continue
        }
        if (JSON.stringify(beforeFields?.[key] ?? null) !== JSON.stringify(afterFields?.[key] ?? null)) {
          return true
        }
      }
      return false
    })()

    if (beforeSnapshot && after && !changed) {
      return result
    }

    const record = new Record(auditLogs, {
      action: "update",
      targetCollection: event.collection.name,
      targetRecordId: event.record?.id || beforeSnapshot?.id || "",
      actorType: actorMeta.actorType,
      actorUser: actorMeta.actorUser,
      actorLabel: actorMeta.actorLabel,
      method: info?.method || "",
      context: info?.context || "",
      ip: typeof event.realIP === "function" ? event.realIP() : "",
      userAgent,
      before: beforeSnapshot,
      after,
    })

    try {
      event.app.save(record)
    } catch (error) {
      console.error("audit_logs update failed", error)
    }
  }

  return result
})

onRecordDeleteRequest((event) => {
  if (!event.collection || !["users", "location", "species", "observation"].includes(event.collection.name)) {
    return event.next()
  }

  const beforeRecord = event.record
  const beforeSnapshot = beforeRecord
    ? {
        id: beforeRecord.id || "",
        targetCollection:
          (typeof beforeRecord.collection === "function" && beforeRecord.collection() && beforeRecord.collection().name) ||
          event.collection.name ||
          "",
        fields: (() => {
          try {
            const raw = typeof beforeRecord.fieldsData === "function" ? beforeRecord.fieldsData() : {}
            return raw === undefined ? undefined : JSON.parse(JSON.stringify(raw))
          } catch (_) {
            return typeof beforeRecord.fieldsData === "function" ? beforeRecord.fieldsData() : {}
          }
        })(),
      }
    : null

  const info = (() => {
    try {
      return typeof event.requestInfo === "function" ? event.requestInfo() : null
    } catch (_) {
      return null
    }
  })()
  const auth = (info && info.auth) || event.auth || null
  const actorCollectionName =
    (auth && typeof auth.collection === "function" && auth.collection() && auth.collection().name) ||
    (auth && typeof auth.isSuperuser === "function" && auth.isSuperuser() ? "_superusers" : "")
  const actorLabel = (() => {
    if (!auth) {
      return "anonymous"
    }
    try {
      if (typeof auth.get === "function") {
        return auth.get("name") || auth.get("email") || auth.id || "anonymous"
      }
    } catch (_) {
      // Fall through.
    }
    return auth.name || auth.email || auth.id || "anonymous"
  })()
  const actorMeta = (() => {
    if (actorCollectionName === "users") {
      return {
        actorType: "user",
        actorUser: auth.id || "",
        actorLabel,
      }
    }

    if (actorCollectionName === "_superusers") {
      return {
        actorType: "superuser",
        actorUser: "",
        actorLabel,
      }
    }

    return {
      actorType: "anonymous",
      actorUser: "",
      actorLabel,
    }
  })()
  const userAgent = info?.headers?.["user-agent"] || info?.headers?.["User-Agent"] || ""
  const result = event.next()
  const auditLogs = event.app.findCollectionByNameOrId("audit_logs")

  if (auditLogs) {
    const record = new Record(auditLogs, {
      action: "delete",
      targetCollection: event.collection.name,
      targetRecordId: event.record?.id || beforeSnapshot?.id || "",
      actorType: actorMeta.actorType,
      actorUser: actorMeta.actorUser,
      actorLabel: actorMeta.actorLabel,
      method: info?.method || "",
      context: info?.context || "",
      ip: typeof event.realIP === "function" ? event.realIP() : "",
      userAgent,
      before: beforeSnapshot,
      after: null,
    })

    try {
      event.app.save(record)
    } catch (error) {
      console.error("audit_logs delete failed", error)
    }
  }

  return result
})
