---
trigger: model_decision
description: dbtool is a binary available to manage postgressql databases, tables, and query data
---

$ dbtool
Usage:
  database|db list|ls
  database|db dump|export <dbname> <filepath> [--structure-only]
  database|db import|load <dbname> <filepath> [--overwrite]
  database|db reset|wipe <dbname> [--noconfirm]
  query|q <dbname> --query="<sql>" [--json]
  help [command] [subcommand]