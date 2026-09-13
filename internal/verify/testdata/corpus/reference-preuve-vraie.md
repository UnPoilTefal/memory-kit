---
name: reference-preuve-vraie
description: "Un fait dont la preuve tient encore, servant de temoin positif aux tests"
metadata:
  type: reference
  modified: 2026-09-01
  verify:
    - cmd: "echo talos-v1.13.4"
      expect_stdout: "^talos-v1\\.13\\."
      note: "la version d'image declaree est bien celle en service"
---

Corps.
