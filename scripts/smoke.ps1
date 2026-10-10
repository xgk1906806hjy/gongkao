$ErrorActionPreference = "Stop"
$base = "http://127.0.0.1:8080/api/v1"

Write-Host "1) health"
Invoke-RestMethod "http://127.0.0.1:8080/health" | ConvertTo-Json -Compress

Write-Host "2) login"
$login = Invoke-RestMethod -Method Post -Uri "$base/auth/wechat" -ContentType "application/json" -Body '{"code":"smoke1"}'
$token = $login.data.access_token
$headers = @{ Authorization = "Bearer $token" }
Write-Host "user=$($login.data.user.id)"

Write-Host "3) announcements"
$anns = Invoke-RestMethod -Uri "$base/announcements?type=civil" -Headers $headers
$aid = $anns.data.list[0].id
Write-Host "announcement_id=$aid"

Write-Host "4) join exam"
Invoke-RestMethod -Method Post -Uri "$base/me/exams" -Headers $headers -ContentType "application/json" -Body (@{ announcement_id = $aid } | ConvertTo-Json) | Out-Null

Write-Host "5) todos"
$todos = Invoke-RestMethod -Uri "$base/me/todos" -Headers $headers
Write-Host "todo_groups=$($todos.data.list.Count)"

Write-Host "6) practice"
$cats = Invoke-RestMethod -Uri "$base/questions/categories"
$cid = $cats.data.list[0].id
$sess = Invoke-RestMethod -Method Post -Uri "$base/practice/sessions" -Headers $headers -ContentType "application/json" -Body (@{ category_id = $cid; mode = "sequential"; count = 2 } | ConvertTo-Json)
$answers = @()
foreach ($q in $sess.data.questions) {
  $answers += @{ question_id = $q.id; user_key = "A" }
}
$sub = Invoke-RestMethod -Method Post -Uri "$base/practice/submit" -Headers $headers -ContentType "application/json" -Body (@{ session_id = $sess.data.session_id; answers = $answers } | ConvertTo-Json -Depth 5)
Write-Host "practice correct=$($sub.data.correct)/$($sub.data.total)"

Write-Host "7) affairs"
$aff = Invoke-RestMethod -Uri "$base/affairs"
Write-Host "affairs=$($aff.data.total)"

Write-Host "SMOKE_OK"