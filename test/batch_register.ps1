$baseUrl = "http://127.0.0.1:8081"
$tokenFile = "C:\Users\EDDIEL\Desktop\code\dspt\test\tokens.txt"
$count = 600  # 注册 600 个用户

# 清空 token 文件
"" | Set-Content $tokenFile

$success = 0

for ($i = 0; $i -lt $count; $i++) {
    $phone = "1380000{0:D4}" -f $i

    # 发验证码
    try {
        Invoke-WebRequest -Uri "$baseUrl/user/code?phone=$phone" `
            -Method POST -ErrorAction Stop | Out-Null
    } catch {
        continue
    }

    # 登录（验证码固定 000000）
    $body = "{`"phone`":`"$phone`",`"code`":`"000000`"}"
    try {
        $resp = Invoke-WebRequest -Uri "$baseUrl/user/login" `
            -Method POST `
            -ContentType "application/json" `
            -Body $body `
            -ErrorAction Stop

        $data = ($resp.Content | ConvertFrom-Json)
        if ($data.success -and $data.data) {
            Add-Content -Path $tokenFile -Value $data.data
            $success++
        }
    } catch {
        # 忽略单个失败
    }

    # 每 50 个打印一次进度
    if ($i % 50 -eq 0) {
        Write-Host "Progress: $i / $count, Success: $success"
    }

    Start-Sleep -Milliseconds 20
}

Write-Host "Complete! Successfully registered $success users, tokens saved in $tokenFile"