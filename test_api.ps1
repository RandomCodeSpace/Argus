# 2. Start Order Service (Air)
Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd 'd:\Development\CodeName Argus\test\orderservice'; air" -WindowStyle Minimized

# 3. Start Payment Service (Air)
Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd 'd:\Development\CodeName Argus\test\paymentservice'; air" -WindowStyle Minimized
$response = Invoke-RestMethod -Uri "http://localhost:8080/api/metrics/dashboard" -Method Get
Write-Host "Dashboard Stats Response:"
$response | Format-List
Write-Host "`nJSON:"
$response | ConvertTo-Json
Stop-Process -Name "orderservice", "paymentservice" -ErrorAction SilentlyContinue -Force