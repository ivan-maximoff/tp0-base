if [ "$#" -ne 2 ]; then
    echo "Uso: $0 <archivo_salida> <cantidad_clientes>"
    exit 1
fi

FILE=$1
CANTIDAD=$2

cat <<EOF > "$FILE"
name: tp0
services:
  server:
    container_name: server
    image: server:latest
    entrypoint: python3 /main.py
    environment:
      - PYTHONUNBUFFERED=1
      - LOGGING_LEVEL=DEBUG
    networks:
      - testing_net

EOF

for i in $(seq 1 "$CANTIDAD"); do
    cat <<EOF >> "$FILE"
  client$i:
    container_name: client$i
    image: client:latest
    entrypoint: /client
    environment:
      - CLI_ID=$i
      - CLI_LOG_LEVEL=DEBUG
    networks:
      - testing_net
    depends_on:
      - server

EOF
done

cat <<EOF >> "$FILE"
networks:
  testing_net:
    ipam:
      driver: default
      config:
        - subnet: 172.25.125.0/24
EOF

echo "Log: Se generó $FILE con $CANTIDAD clientes."