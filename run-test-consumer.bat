echo [2/3] Verificando conexoes...
echo - RabbitMQ: 34.71.212.239:5672
echo - Redis: 35.199.96.88:6379
echo.

echo [3/3] Iniciando consumer...
echo.
echo ============================================================================
echo   Pressione Ctrl+C para parar
echo ============================================================================
echo.

python test-consumer.py

pause
