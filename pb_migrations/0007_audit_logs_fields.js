/// <reference path="../pb_data/types.d.ts" />
migrate((app) => {
  const adminOrSuperuser = "@request.auth.collectionName = \"_superusers\" || @request.auth.role = \"admin\""
  const existing = app.findCollectionByNameOrId("audit_logs")
  if (existing) {
    app.delete(existing)
  }

  const auditLogs = new Collection({ type: "base", name: "audit_logs" })
  auditLogs.listRule = adminOrSuperuser
  auditLogs.viewRule = adminOrSuperuser
  auditLogs.createRule = adminOrSuperuser
  auditLogs.updateRule = "@request.auth.id = \"\""
  auditLogs.deleteRule = "@request.auth.id = \"\""
  auditLogs.fields.add(
    new SelectField({
      name: "action",
      required: true,
      maxSelect: 1,
      values: ["create", "update", "delete"],
      presentable: true,
    }),
    new TextField({
      name: "targetCollection",
      required: true,
      max: 100,
      presentable: true,
    }),
    new TextField({
      name: "targetRecordId",
      required: true,
      max: 100,
      presentable: true,
    }),
    new SelectField({
      name: "actorType",
      required: true,
      maxSelect: 1,
      values: ["user", "superuser", "anonymous"],
      presentable: true,
    }),
    new RelationField({
      name: "actorUser",
      collectionId: app.findCollectionByNameOrId("users").id,
      maxSelect: 1,
      required: false,
      presentable: true,
    }),
    new TextField({
      name: "actorLabel",
      required: true,
      max: 200,
      presentable: true,
    }),
    new TextField({
      name: "method",
      required: false,
      max: 20,
      presentable: true,
    }),
    new TextField({
      name: "context",
      required: false,
      max: 50,
      presentable: true,
    }),
    new TextField({
      name: "ip",
      required: false,
      max: 100,
      presentable: true,
    }),
    new TextField({
      name: "userAgent",
      required: false,
      max: 500,
      presentable: true,
    }),
    new JSONField({
      name: "before",
      required: false,
      maxSize: 1024 * 1024,
      presentable: false,
    }),
    new JSONField({
      name: "after",
      required: false,
      maxSize: 1024 * 1024,
      presentable: false,
    }),
    new AutodateField({
      name: "loggedAt",
      onCreate: true,
      onUpdate: false,
      presentable: true,
    }),
  )

  app.save(auditLogs)
}, (app) => {
  try {
    const auditLogs = app.findCollectionByNameOrId("audit_logs")
    if (auditLogs) {
      app.delete(auditLogs)
    }
  } catch (_) {
    // Ignore missing collection during rollback.
  }
})
