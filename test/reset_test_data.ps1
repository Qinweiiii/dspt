$voucherId = 8
$initStock = 500
$password = "root1234"

Write-Host "Reseting Redis..."
redis-cli SET seckill:stock:$voucherId $initStock
redis-cli DEL seckill:order:$voucherId
redis-cli XTRIM stream.orders MAXLEN 0

Write-Host "Reseting MySQL..."
mysql -u root -p$password qppt -e "UPDATE tb_seckill_voucher SET stock=$initStock WHERE voucher_id=$voucherId;"
mysql -u root -p$password qppt -e "DELETE FROM tb_voucher_order WHERE voucher_id=$voucherId;"

Write-Host "Reseting complete! Initial stock = $initStock"