@{
    "F-01" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-foundation.md"
        DependsOn = @()
        Locks     = @()
    }
    "F-02" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-foundation.md"
        DependsOn = @("F-01")
        Locks     = @("routes")
    }
    "F-03" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-foundation.md"
        DependsOn = @("F-02")
        Locks     = @("routes", "config", "i18n")
    }
    "F-04" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-foundation.md"
        DependsOn = @("F-03")
        Locks     = @("routes", "config", "i18n")
    }
    "F-05" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-foundation.md"
        DependsOn = @("F-02")
        Locks     = @("migration-registry", "agent-service")
    }
    "F-06" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-foundation.md"
        DependsOn = @("F-02")
        Locks     = @("config", "i18n", "content-detail")
    }
    "V-01" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-verification-feedback.md"
        DependsOn = @("F-03")
        Locks     = @("config", "migration-registry")
    }
    "V-02" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-verification-feedback.md"
        DependsOn = @("V-01")
        Locks     = @("routes", "config")
    }
    "V-03" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-verification-feedback.md"
        DependsOn = @("V-02")
        Locks     = @("i18n")
    }
    "V-04" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-verification-feedback.md"
        DependsOn = @("V-03")
        Locks     = @("i18n", "footer")
    }
    "V-05" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-verification-feedback.md"
        DependsOn = @("V-01")
        Locks     = @("routes", "config", "migration-registry")
    }
    "V-06" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-verification-feedback.md"
        DependsOn = @("V-04", "V-05")
        Locks     = @("i18n", "footer")
    }
    "A-01" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-admin-operations.md"
        DependsOn = @("F-02")
        Locks     = @("migration-registry")
    }
    "A-02" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-admin-operations.md"
        DependsOn = @("A-01")
        Locks     = @("routes")
    }
    "A-03" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-admin-operations.md"
        DependsOn = @("A-04")
        Locks     = @("i18n", "admin-subtree")
    }
    "A-04" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-admin-operations.md"
        DependsOn = @("A-02", "V-05")
        Locks     = @("routes", "i18n", "admin-subtree")
    }
    "A-05" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-admin-operations.md"
        DependsOn = @("A-03", "A-04")
        Locks     = @("routes", "i18n", "admin-subtree")
    }
    "G-01" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-agent-entrypoints.md"
        DependsOn = @("F-04")
        Locks     = @("i18n", "content-detail", "publish-form")
    }
    "G-02" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-agent-entrypoints.md"
        DependsOn = @("F-05", "G-01")
        Locks     = @("i18n", "agent-service")
    }
    "G-03" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-agent-entrypoints.md"
        DependsOn = @("G-01")
        Locks     = @("config", "i18n", "agent-service")
    }
    "G-04" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-agent-entrypoints.md"
        DependsOn = @("G-03")
        Locks     = @("i18n", "content-detail")
    }
    "G-05" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-agent-entrypoints.md"
        DependsOn = @("G-01")
        Locks     = @("i18n", "publish-form", "agent-service")
    }
    "D-01" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-desktop-deploy-security.md"
        DependsOn = @("G-01")
        Locks     = @("routes", "config")
    }
    "D-02" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-desktop-deploy-security.md"
        DependsOn = @("D-01", "F-06")
        Locks     = @("routes", "config")
        Desktop   = $true
    }
    "D-03" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-desktop-deploy-security.md"
        DependsOn = @("D-02")
        Locks     = @("routes", "config", "agent-service", "desktop-schema")
        Desktop   = $true
    }
    "D-04" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-desktop-deploy-security.md"
        DependsOn = @("D-03")
        Locks     = @("desktop-schema")
        Desktop   = $true
    }
    "D-05" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-desktop-deploy-security.md"
        DependsOn = @("D-04")
        Locks     = @("i18n", "content-detail", "desktop-schema")
        Desktop   = $true
    }
    "R-01" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-release-validation.md"
        DependsOn = @("V-06", "A-05", "G-05", "F-06", "D-01")
        Locks     = @("release-validation")
    }
    "R-02" = @{
        Plan      = "docs/superpowers/plans/2026-05-30-omnicraft-beta-release-validation.md"
        DependsOn = @("D-05")
        Locks     = @("release-validation", "desktop-schema")
        Desktop   = $true
    }
}
