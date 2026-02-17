$ErrorActionPreference = "Stop"
Stop-Process -Name "orderservice", "paymentservice" -ErrorAction SilentlyContinue -Force
# 2. Start Order Service (Air)
Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd 'd:\Development\CodeName Argus\test\orderservice'; air" -WindowStyle Minimized

# 3. Start Payment Service (Air)
Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd 'd:\Development\CodeName Argus\test\paymentservice'; air" -WindowStyle Minimized

Write-Host "Starting ARGUS Load Test..." -ForegroundColor Cyan

$endpoint = "http://localhost:9001/order"
$iterations = 17
$concurrent = 19


$jobs = @()

for ($i = 0; $i -lt $concurrent; $i++) {
    $jobs += Start-Job -ScriptBlock {
        param($endpoint, $iterations)
        $ErrorActionPreference = "SilentlyContinue"
        for ($j = 0; $j -lt $iterations; $j++) {
            try {
                $response = Invoke-WebRequest -Uri $endpoint -Method Post
                if ($response.StatusCode -ne 200) {
                    Write-Output "Error: $($response.StatusCode)"
                }
            }
            catch {
                Write-Output "Error: $($_.Exception.Message)"
            }
            Start-Sleep -Milliseconds 100
        }
    } -ArgumentList $endpoint, $iterations
}

Write-Host "$concurrent jobs started. Waiting for completion..."
Wait-Job -Job $jobs | Out-Null
Receive-Job -Job $jobs | Group-Object | Format-Table Count, Name -AutoSize
Stop-Process -Name "orderservice", "paymentservice" -ErrorAction SilentlyContinue -Force
Write-Host "Load test complete." -ForegroundColor Green
