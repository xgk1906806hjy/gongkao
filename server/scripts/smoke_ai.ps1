param([string]$Base = "http://127.0.0.1:8080/api/v1")
$ErrorActionPreference = "Stop"
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

function Post($path, $body, $token) {
  $json = $body | ConvertTo-Json -Depth 6 -Compress
  $bytes = [System.Text.Encoding]::UTF8.GetBytes($json)
  $h = @{}
  if ($token) { $h.Authorization = "Bearer $token" }
  (Invoke-RestMethod -Method POST -Uri ($Base + $path) -ContentType "application/json; charset=utf-8" -Headers $h -Body $bytes).data
}
function Get($path, $token) {
  (Invoke-RestMethod -Uri ($Base + $path) -Headers @{ Authorization = "Bearer $token" }).data
}

$login = Post "/auth/wechat" @{ code = "smoke-ai-" + (Get-Date -Format "HHmmss") } $null
$t = $login.access_token

$status = Get "/ai/status" $t
"status: llm=$($status.llm_enabled) model=$($status.model) used=$($status.used_today)/$($status.daily_limit)"

$resume = @"
李四 湖南师范大学 法学专业 本科 2027届应届毕业生
政治面貌：中共党员
证书：法律职业资格、英语六级
求职意向：意向地区 湖南
"@
$r = Post "/ai/resume/analyze" @{ resume_text = $resume } $t
"resume: provider=$($r.provider) report=$($r.report_id) edu=$($r.profile.education) major=$($r.profile.major) fresh=$($r.profile.fresh_graduate) regions=$($r.profile.target_regions)"
"summary: $($r.summary)"
"directions: " + (($r.directions | ForEach-Object { $_.name }) -join " | ")
$r.recommendations | Select-Object -First 3 | ForEach-Object { "  rec [$($_.score)] $($_.title) :: $($_.reasons -join '; ')" }

$p = Post "/ai/study-plan" @{ daily_hours = 4 } $t
"plan: provider=$($p.provider) exam=$($p.exam_date) days=$($p.days_left) note=$($p.exam_date_note) phases=$($p.phases.Count) weeks=$($p.weekly.Count)"
$p.daily_template | ForEach-Object { "  $($_.module) $($_.minutes)min - $($_.task)" }

$a = Post "/ai/ask" @{ question = "行测时间怎么分配？" } $t
"ask: provider=$($a.provider) answer=$($a.answer.Substring(0, [Math]::Min(60, $a.answer.Length)))"

$reports = Get "/me/ai/reports" $t
"reports total=$($reports.total)"
$d = Get ("/me/ai/reports/" + $r.report_id) $t
"report detail kind=$($d.kind) recs=$($d.result.recommendations.Count)"
