param(
    [int]$TotalRequests = 600,
    [int]$Concurrency = 200,
    [string]$VoucherId = "8",
    [string]$BaseUrl = "http://127.0.0.1:8081",
    [string]$TokenFile = "C:\Users\EDDIEL\Desktop\code\dspt\test\tokens.txt"
)

# 读 token
$tokens = Get-Content $TokenFile | Where-Object { $_ -ne "" }
Write-Host "Loaded $($tokens.Count) tokens"

if ($tokens.Count -eq 0) {
    Write-Host "Error: token file is empty"
    exit 1
}

# 用 .NET HttpClient 做真正的异步并发
Add-Type -TypeDefinition @"
using System;
using System.Net.Http;
using System.Threading.Tasks;
using System.Collections.Concurrent;

public class SeckillTester {
    private static readonly HttpClient client = new HttpClient();
    
    public static int[] RunTest(string[] tokens, string url, int total, int concurrency) {
        var successCount = 0;
        var failCount = 0;
        var results = new ConcurrentBag<string>();
        
        var semaphore = new System.Threading.SemaphoreSlim(concurrency);
        var tasks = new Task[total];
        
        for (int i = 0; i < total; i++) {
            var token = tokens[i % tokens.Length];
            var idx = i;
            tasks[i] = Task.Run(async () => {
                await semaphore.WaitAsync();
                try {
                    var request = new HttpRequestMessage(HttpMethod.Post, url);
                    request.Headers.Add("authorization", token);
                    var resp = await client.SendAsync(request);
                    var body = await resp.Content.ReadAsStringAsync();
                    if (body.Contains("\"success\":true")) {
                        System.Threading.Interlocked.Increment(ref successCount);
                    } else {
                        System.Threading.Interlocked.Increment(ref failCount);
                        if (failCount <= 5) {
                            Console.WriteLine("FAIL: " + body);
                        }
                    }
                } catch {
                    System.Threading.Interlocked.Increment(ref failCount);
                } finally {
                    semaphore.Release();
                }
            });
        }
        
        Task.WaitAll(tasks);
        return new int[] { successCount, failCount };
    }
}
"@ -ReferencedAssemblies System.Net.Http

$url = "$BaseUrl/voucher-order/seckill/$VoucherId"
Write-Host "Start: Total=$TotalRequests, Concurrency=$Concurrency"
Write-Host "URL: $url"
Write-Host "--------------------------------------"

$startTime = Get-Date
$result = [SeckillTester]::RunTest($tokens, $url, $TotalRequests, $Concurrency)
$elapsed = ((Get-Date) - $startTime).TotalSeconds

Write-Host ""
Write-Host "====== Results ======"
Write-Host "Total Requests:      $TotalRequests"
Write-Host "Successful:          $($result[0])"
Write-Host "Failed/Blocked:      $($result[1])"
Write-Host "Elapsed:             $([Math]::Round($elapsed, 2)) secs"
Write-Host "QPS:                 $([Math]::Round($TotalRequests / $elapsed, 0)) req/s"