import argparse
import threading
import time
import json
import logging
import random
from collections import deque
from flask import Flask, Response, render_template_string
import pika
import redis

# Configuração de Logs
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

# Lista de Routing Keys possiveis (para amostragem aleatoria)
ALL_ROUTING_KEYS = [
    "t3-labs.cam1",
    "t3-labs.cam-mercado",
    "t3-labs.pix1", "t3-labs.pix2", "t3-labs.pix3", "t3-labs.pix4", "t3-labs.pix5",
    "t3-labs.shop1", "t3-labs.shop2", "t3-labs.shop3", "t3-labs.shop5"
]

# Estado Global
latest_frames = {} # {camera_id: jpeg_bytes}
frame_stats = {}   # {camera_id: dict}
# Estrutura de stats:
# {
#   "camera_id": {
#       "size": int,
#       "latency_ms": float,
#       "fps": float,
#       "bitrate_mbps": float,
#       "total_frames": int,
#       "timestamp": float (local recv time)
#   }
# }

# Histórico para cálculos de janela (FPS/Bitrate)
history = {} # {camera_id: deque([(time, bytes), ...])}
HISTORY_WINDOW = 2.0 # Segundos para calcular média móvel

stats_lock = threading.Lock()

app = Flask(__name__)

# HTML Template
HTML_TEMPLATE = """
<!DOCTYPE html>
<html>
<head>
    <title>Edge Video Debugger (Stress Test Mode)</title>
    <style>
        body { font-family: 'Segoe UI', sans-serif; background: #1a1a1a; color: #f0f0f0; margin: 0; padding: 20px; }
        .camera-grid { display: flex; flex-wrap: wrap; gap: 20px; }
        .camera-card { 
            background: #2a2a2a; border-radius: 12px; overflow: hidden; 
            box-shadow: 0 4px 6px rgba(0,0,0,0.3); border: 1px solid #333;
            max-width: 640px;
        }
        .header { 
            padding: 10px 15px; background: #333; font-weight: bold; 
            display: flex; justify-content: space-between; align-items: center;
        }
        .stats-panel { 
            padding: 10px 15px; background: #222; font-family: 'Consolas', monospace; font-size: 0.85em;
            display: grid; grid-template-columns: 1fr 1fr; gap: 5px; color: #aaa;
        }
        .stat-item { display: flex; justify-content: space-between; }
        .stat-val { color: #fff; font-weight: bold; }
        .warn { color: #f55; }
        .good { color: #5f5; }
        img { width: 100%; display: block; }
        a.refresh { 
            display: inline-block; margin-bottom: 20px; padding: 10px 20px; 
            background: #007acc; color: white; text-decoration: none; border-radius: 4px; 
        }
        .info-bar { 
            background: #444; padding: 10px; margin-bottom: 20px; border-radius: 4px; border-left: 5px solid #ffcc00;
        }
    </style>
    <script>
        setInterval(function() {
            fetch('/stats').then(r => r.json()).then(data => {
                for (let cam in data) {
                    let s = data[cam];
                    
                    // Update stats
                    let elSize = document.getElementById('size-' + cam);
                    if (elSize) elSize.innerText = (s.size/1024).toFixed(1) + ' KB';
                    
                    let elFps = document.getElementById('fps-' + cam);
                    if (elFps) elFps.innerText = s.fps.toFixed(1);
                    
                    let elBit = document.getElementById('bitrate-' + cam);
                    if (elBit) elBit.innerText = s.bitrate_mbps.toFixed(2) + ' Mbps';
                    
                    let elFrames = document.getElementById('frames-' + cam);
                    if (elFrames) elFrames.innerText = s.total_frames;
                    
                    let latEl = document.getElementById('latency-' + cam);
                    if (latEl) {
                        latEl.innerText = s.latency_ms.toFixed(1) + ' ms';
                        latEl.className = 'stat-val ' + (s.latency_ms > 200 ? 'warn' : 'good');
                    }
                }
            });
        }, 500);
    </script>
</head>
<body>
    <h1>🎥 Edge Video Debugger (Stress Test Samping)</h1>
    <div class="info-bar">
        Mostrando amostra de 2 câmeras aleatórias para evitar sobrecarga de rede no monitoramento.
        <br>O Producer continua enviando todas as 11 câmeras.
    </div>
    <a href="/" class="refresh">🔄 Sortear Novas Câmeras</a>
    
    <div class="camera-grid">
    {% for cam in cameras %}
    <div class="camera-card">
        <div class="header">
            <span>{{ cam }}</span>
            <span style="font-size: 0.8em; color: #888;">LIVE</span>
        </div>
        <img src="/video_feed/{{ cam }}" />
        <div class="stats-panel">
            <div class="stat-item"><span>FPS:</span> <span id="fps-{{ cam }}" class="stat-val">--</span></div>
            <div class="stat-item"><span>Bitrate:</span> <span id="bitrate-{{ cam }}" class="stat-val">--</span></div>
            <div class="stat-item"><span>Latency:</span> <span id="latency-{{ cam }}" class="stat-val">--</span></div>
            <div class="stat-item"><span>Frame Size:</span> <span id="size-{{ cam }}" class="stat-val">--</span></div>
            <div class="stat-item"><span>Total Frames:</span> <span id="frames-{{ cam }}" class="stat-val">--</span></div>
        </div>
    </div>
    {% else %}
        <p>Aguardando frames do RabbitMQ para as chaves sorteadas...</p>
    {% endfor %}
    </div>
</body>
</html>
"""

def rabbitmq_worker(amqp_url, redis_host, redis_port, redis_password):
    """Worker que escuta RabbitMQ e busca frames no Redis - Com Reconexão e Filtragem"""
    logger.info(f"Conectando ao RabbitMQ: {amqp_url}")
    logger.info(f"Conectando ao Redis: {redis_host}:{redis_port}")

    # Sorteia 2 chaves para monitorar
    target_keys = random.sample(ALL_ROUTING_KEYS, 2)
    logger.info(f"🎯 Monitorando apenas: {target_keys}")

    r_client = redis.Redis(host=redis_host, port=redis_port, password=redis_password, db=0)

    while True:
        try:
            params = pika.URLParameters(amqp_url)
            # Aumentar timeout de heartbeat para evitar resets em rede lenta
            params.heartbeat = 60
            params.blocked_connection_timeout = 300
            
            connection = pika.BlockingConnection(params)
            channel = connection.channel()

            # Declara Exchange (Idempotente)
            channel.exchange_declare(exchange='t3-labs.exchange', exchange_type='topic', durable=True)

            # Cria Fila Temporária Exclusiva
            result = channel.queue_declare(queue='', exclusive=True)
            queue_name = result.method.queue

            # Bind APENAS nas chaves sorteadas (Filtragem Server-Side)
            for key in target_keys:
                channel.queue_bind(exchange='t3-labs.exchange', queue=queue_name, routing_key=key)
            
            logger.info("✓ Worker conectado e aguardando frames...")

            def callback(ch, method, properties, body):
                try:
                    # O body é a redis_key (ex: t3-labs:frames:camera-corredor:1769305972554001200)
                    redis_key = body.decode('utf-8')
                    recv_time = time.time()
                    
                    camera_id = "unknown"
                    if properties.headers and 'camera_id' in properties.headers:
                        camera_id = properties.headers['camera_id']
                    
                    # Se não veio header, tenta parsear
                    producer_ts_nanos = 0
                    if camera_id == "unknown":
                        parts = redis_key.split(':')
                        if len(parts) >= 4:
                            camera_id = parts[2]
                            producer_ts_nanos = int(parts[3])
                    
                    # Busca frame do Redis
                    frame_data = r_client.get(redis_key)
                    
                    if frame_data:
                        frame_size = len(frame_data)
                        
                        latency_ms = 0
                        # Re-tenta parsear TS se não veio
                        if producer_ts_nanos == 0:
                             parts = redis_key.split(':')
                             if len(parts) >= 4:
                                 producer_ts_nanos = int(parts[3])

                        if producer_ts_nanos > 0:
                            producer_time_sec = producer_ts_nanos / 1e9
                            latency_ms = (recv_time - producer_time_sec) * 1000
                        
                        with stats_lock:
                            latest_frames[camera_id] = frame_data
                            
                            # Inicializa histórico
                            if camera_id not in history:
                                history[camera_id] = deque()
                            
                            history[camera_id].append((recv_time, frame_size))
                            
                            while history[camera_id] and history[camera_id][0][0] < recv_time - HISTORY_WINDOW:
                                history[camera_id].popleft()
                                
                            dq = history[camera_id]
                            count = len(dq)
                            fps = 0
                            bitrate_mbps = 0
                            
                            if count > 1:
                                duration = dq[-1][0] - dq[0][0]
                                if duration > 0:
                                    fps = (count - 1) / duration
                                    total_bytes = sum(item[1] for item in dq)
                                    bitrate_mbps = (total_bytes * 8) / (duration * 1_000_000)
                                    
                            total = frame_stats.get(camera_id, {}).get("total_frames", 0) + 1
                            
                            frame_stats[camera_id] = {
                                "size": frame_size,
                                "latency_ms": latency_ms,
                                "fps": fps,
                                "bitrate_mbps": bitrate_mbps,
                                "total_frames": total,
                                "timestamp": recv_time
                            }
                            
                except Exception as e:
                    logger.error(f"Erro processando frame: {e}")

            channel.basic_consume(queue=queue_name, on_message_callback=callback, auto_ack=True)
            channel.start_consuming()

        except (pika.exceptions.AMQPConnectionError, pika.exceptions.ConnectionClosedByBroker, pika.exceptions.StreamLostError) as e:
            logger.error(f"⚠️ Conexão perdida ({e}). Reconectando em 2s...")
            time.sleep(2)
        except Exception as e:
            logger.error(f"Erro fatal worker: {e}. Retry em 5s...")
            time.sleep(5)

@app.route('/')
def index():
    with stats_lock:
        cameras = list(latest_frames.keys())
        cameras.sort()
    return render_template_string(HTML_TEMPLATE, cameras=cameras)

@app.route('/stats')
def stats():
    with stats_lock:
        return json.dumps(frame_stats)

def gen(camera_id):
    """Gerador de stream MJPEG"""
    while True:
        frame = None
        with stats_lock:
            if camera_id in latest_frames:
                frame = latest_frames[camera_id]
        
        if frame:
            yield (b'--frame\r\n'
                   b'Content-Type: image/jpeg\r\n\r\n' + frame + b'\r\n')
        
        time.sleep(0.06)

@app.route('/video_feed/<camera_id>')
def video_feed(camera_id):
    return Response(gen(camera_id),
                    mimetype='multipart/x-mixed-replace; boundary=frame')

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Debug Consumer')
    parser.add_argument('--redis-host', default='34.30.59.236', help='Redis Host')
    parser.add_argument('--redis-port', type=int, default=6379, help='Redis Port')
    parser.add_argument('--redis-password', default='MinhaSenhaMuitoForte@2024', help='Redis Password')
    
    DEFAULT_AMQP = "amqp://t3-labs:vTUaGG9S0Kbl_1AhqPxudML1jCYHRq0efkRDA6p_lgM@34.71.212.239:5672/t3-labs"
    parser.add_argument('--amqp-url', default=DEFAULT_AMQP, help='RabbitMQ URL')
    
    args = parser.parse_args()

    t = threading.Thread(target=rabbitmq_worker, args=(args.amqp_url, args.redis_host, args.redis_port, args.redis_password))
    t.daemon = True
    t.start()

    logger.info("Iniciando Web Server na porta 5000...")
    app.run(host='0.0.0.0', port=5000, debug=False)
