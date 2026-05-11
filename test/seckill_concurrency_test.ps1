$baseUrl = "http://localhost:8081"
$voucherId = 8
$tokenFile = "C:\Users\EDDIEL\Desktop\code\dspt\test\tokens.txt"
$concurrency = 200   # 并发数
$totalRequests = 600 # 总请求数

# 读取所有 token
$tokens = Get-Content $tokenFile | Where-Object { $_ -ne "" }
Write-Host "Loaded $($tokens.Count)  token"

if ($tokens.Count -eq 0) {
    Write-Host "Error: token file is empty, please register first!"
    exit 1
}

# 统计变量
$successCount = 0
$failCount = 0
$startTime = Get-Date
# $results = [System.Collections.Concurrent.ConcurrentBag[string]]::new()
$results = New-Object System.Collections.Concurrent.ConcurrentBag[string]

# 把请求拆成批次，每批 $concurrency 个并发
# $batches = [System.Collections.Generic.List[object]]::new()
$batches = New-Object System.Collections.Generic.List[object]
for ($i = 0; $i -lt $totalRequests; $i += $concurrency) {
    $batchEnd = [Math]::Min($i + $concurrency, $totalRequests)
    $batches.Add(@{Start = $i; End = $batchEnd})
}

Write-Host "Start to pressure test: Total requests=$totalRequests, Concurrency=$concurrency, Batches=$($batches.Count)"
Write-Host "--------------------------------------"

foreach ($batch in $batches) {
    $jobs = @()

    for ($j = $batch.Start; $j -lt $batch.End; $j++) {
        $token = $tokens[$j % $tokens.Count]
        $uri = "$baseUrl/voucher-order/seckill/$voucherId"

        # 每个请求起一个 Job
        $jobs += Start-Job -ScriptBlock {
            param($uri, $token)
            try {
                # $resp = Invoke-WebRequest -Uri $uri `
                #     -Method POST `
                #     -Headers @{ "authorization" = $token } `
                #     -ErrorAction Stop
                $resp = Invoke-WebRequest -Uri $uri -Method POST -Headers @{"authorization" = $token} -ErrorAction Stop -UseBasicParsing
                $body = $resp.Content | ConvertFrom-Json
                if ($body.success) { "success" } else { "fail:$($body.errorMsg)" }
            } catch {
                "error:$($_.Exception.Message)"
            }
        } -ArgumentList $uri, $token
    }

    # 等待这批完成
    # $jobs | Wait-Job | Out-Null
    while ($jobs | Where-Object { $_.State -eq 'Running' }) {
        Start-Sleep -Milliseconds 100
    }

    # 收集结果
    foreach ($job in $jobs) {
        $result = Receive-Job $job
        $results.Add($result)
        if ($result -eq "success") { $script:successCount++ }
        else { $script:failCount++ }
        Remove-Job $job
    }

    Write-Host "Finished $($batch.End) / $totalRequests (Success: $successCount) "
}

$elapsed = ((Get-Date) - $startTime).TotalSeconds
Write-Host ""
Write-Host "====== Pressure Test Results ======"
Write-Host "Total Requests:   $totalRequests"
Write-Host "Successful Purchases:   $successCount"
Write-Host "Failed/Blocked:  $failCount"
Write-Host "Elapsed Time:       $([Math]::Round($elapsed, 2)) seconds"
Write-Host "Average QPS:   $([Math]::Round($totalRequests / $elapsed, 0)) req/s"