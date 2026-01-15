# Database Migrations

This directory contains versioned SQL migration files for the Preempt database schema.

## Overview

We use [golang-migrate](https://github.com/golang-migrate/migrate) to manage database schema changes. Migrations are version-controlled and can be applied incrementally or rolled back.

## Migration Files

Migration files come in pairs:
- `XXXXXX_name.up.sql` - Applied when migrating forward
- `XXXXXX_name.down.sql` - Applied when rolling back

Where `XXXXXX` is a sequential version number (e.g., `000001`, `000002`, etc.).

## Current Migrations

1. **000001_initial_schema** - Creates initial tables:
   - `metrics` - Weather metrics data
   - `anomalies` - Detected anomalies
   - `alarm_suggestions` - ML-generated alarm suggestions

## Usage

### Apply All Pending Migrations
```bash
make migrate-up
```

### Rollback Last Migration
```bash
make migrate-down
```

### Rollback All Migrations
```bash
make migrate-down-all
```

### Create New Migration
```bash
make migrate-create NAME=add_locations_table
```

This creates:
- `migrations/XXXXXX_add_locations_table.up.sql`
- `migrations/XXXXXX_add_locations_table.down.sql`

### Check Current Migration Version
```bash
make migrate-version
```

### Force Set Migration Version (for recovery)
```bash
make migrate-force VERSION=1
```

## Docker Compose Integration

The `migrate` service in `docker-compose.yml` automatically runs pending migrations on startup. It:
1. Waits for MySQL to be healthy
2. Applies all pending migrations
3. Exits (restart: on-failure)

## Best Practices

### Writing Migrations

1. **Make migrations atomic** - Each migration should do one logical thing
2. **Always provide down migrations** - Every up must have a corresponding down
3. **Test rollbacks** - Verify down migrations work before committing
4. **Use transactions where possible** - Wrap DDL in transactions when supported
5. **Avoid data migrations in schema migrations** - Keep them separate when possible

### Example Migration Structure

**up.sql:**
```sql
-- Add new column
ALTER TABLE metrics ADD COLUMN source VARCHAR(50) NOT NULL DEFAULT 'open-meteo';

-- Create index
CREATE INDEX idx_metrics_source ON metrics(source);
```

**down.sql:**
```sql
-- Drop index first
DROP INDEX idx_metrics_source ON metrics;

-- Remove column
ALTER TABLE metrics DROP COLUMN source;
```

### Migration Workflow

1. **Create migration**
   ```bash
   make migrate-create NAME=add_feature
   ```

2. **Write up/down SQL** - Edit the generated files

3. **Test locally**
   ```bash
   make migrate-up      # Apply
   make migrate-down    # Rollback
   make migrate-up      # Re-apply
   ```

4. **Commit** - Add migration files to git

5. **Deploy** - Migrations run automatically via docker-compose

## Troubleshooting

### Migration Failed Mid-way

If a migration fails partway through:
```bash
# Check current version
make migrate-version

# If dirty state, fix manually and force version
make migrate-force VERSION=X
```

### Schema Drift

All schema changes MUST go through migrations. Never:
- Modify `init.sql` (deprecated)
- Run manual SQL against production
- Use migrations in one env but manual SQL in another

### Starting Fresh

To completely reset the database:
```bash
docker-compose down -v  # Removes volumes
docker-compose up       # Recreates DB and runs migrations
```

## Migration Strategy for Production

1. **Blue-Green Deployments**: 
   - New schema must be backward compatible
   - Deploy code that works with both old and new schema
   - Run migration
   - Deploy code that uses new schema

2. **Rolling Updates**:
   - Additive changes only (new columns, tables)
   - Use default values for new columns
   - Never drop columns in same release as code changes

3. **Large Datasets**:
   - For huge tables, use online schema change tools
   - Consider pt-online-schema-change or gh-ost
   - Test migration time on production-sized data

## References

- [golang-migrate documentation](https://github.com/golang-migrate/migrate)
- [Database migration best practices](https://www.brunton-spall.co.uk/post/2014/05/06/database-migrations-done-right/)
