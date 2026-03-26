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
    build:
      context: .
      dockerfile: ./server/Dockerfile
    image: server:latest
    entrypoint: python3 /main.py
    environment:
      - CAN_AGENCIES=$CANTIDAD
      - PYTHONUNBUFFERED=1
    networks:
      - testing_net
    volumes:
      - ./server/config.ini:/config.ini

EOF

for i in $(seq 1 "$CANTIDAD"); do
    cat <<EOF >> "$FILE"
  client$i:
    container_name: client$i
    build:
      context: .
      dockerfile: ./client/Dockerfile
    image: client:latest
    entrypoint: /client
    environment:
      - CLI_ID=$i
      - NOMBRE=Agencia_$i
      - APELLIDO=Lotería
      - DOCUMENTO=3090446$i
      - NACIMIENTO=1999-03-17
      - NUMERO=7574
    networks:
      - testing_net
    depends_on:
      - server
    volumes:
      - ./client/config.yaml:/config.yaml
      - ./.data:/data
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