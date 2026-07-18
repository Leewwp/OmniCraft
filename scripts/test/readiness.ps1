[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Invoke-DefaultWebRequest {
    param([string] $Uri)
    return Invoke-WebRequest -Uri $Uri -UseBasicParsing -TimeoutSec 5
}

function Invoke-Readiness {
    [CmdletBinding()]
    param(
        [scriptblock] $Request = ${function:Invoke-DefaultWebRequest},
        [scriptblock] $ProcessProbe = { param($Process) $null -ne $Process -and -not $Process.HasExited },
        $BackendProcess,
        $FrontendProcess,
        [string] $BackendUrl = "http://127.0.0.1:8080",
        [string] $FrontendUrl = "http://127.0.0.1:3000"
    )
    $checks = @(
        @{ Name = "backend process"; Run = { if ($null -ne $BackendProcess -and -not (& $ProcessProbe $BackendProcess)) { throw "backend process exited" } } },
        @{ Name = "frontend process"; Run = { if ($null -ne $FrontendProcess -and -not (& $ProcessProbe $FrontendProcess)) { throw "frontend process exited" } } },
        @{ Name = "backend healthz"; Run = { $r = & $Request "$BackendUrl/healthz"; if ($r.StatusCode -lt 200 -or $r.StatusCode -ge 300) { throw "unexpected HTTP $($r.StatusCode)" } } },
        @{ Name = "frontend page"; Run = { $r = & $Request "$FrontendUrl/"; if ($r.StatusCode -lt 200 -or $r.StatusCode -ge 400) { throw "unexpected HTTP $($r.StatusCode)" } } },
        @{ Name = "real public config API"; Run = { $r = & $Request "$BackendUrl/api/v1/config/public"; $body = $r.Content | ConvertFrom-Json; if ($null -eq $body.features -or $null -eq $body.captcha) { throw "required public config fields are missing" } } }
    )
    foreach ($check in $checks) {
        try { & $check.Run } catch { return [pscustomobject]@{ Status = "BLOCKED"; ExitCode = 2; Detail = "$($check.Name): $($_.Exception.Message)" } }
    }
    return [pscustomobject]@{ Status = "PASS"; ExitCode = 0; Detail = "backend, frontend, and real public API are ready" }
}

if ($MyInvocation.InvocationName -ne ".") {
    $result = Invoke-Readiness
    Write-Host "READINESS $($result.Status): $($result.Detail)"
    exit $result.ExitCode
}
