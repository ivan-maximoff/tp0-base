# Resolución TP0

### Ejercicio 1: Generador de Docker Compose

Creación de un script en Bash que utiliza un bucle seq para crear los contenedores dinámicamente.

**Comando:**

```bash
chmod +x generar-compose.sh
./generar-compose.sh docker-compose-dev.yaml 5
```

### Ejercicio 2: Persistencia con Volúmenes

Ahora se inyectan los archivos de configuración con Docker Volumes y se eliminaron las variables de entorno hardcodeadas ahora definidas mediante los archivos de configuración.

### Ejercicio 3: Validación del Echo Server

Creación de un script en Bash que levanta un contenedor temporal de BusyBox (contenedor liviano con comandos típicos de la shell) conectado a la red del servidor para enviarle un mensaje vía netcat y validar que efectivamente sea un echo server.

**Comando:**

```bash
chmod +x validar-echo-server.sh
./validar-echo-server.sh
```

### Ejercicio 4: Graceful Shutdown

Implementación de manejadores de señales SIGTERM en ambos lenguajes para asegurar el cierre correcto de recursos antes de finalizar los procesos. En Python se cerró el socket para interrumpir el accept() y en Go se utilizó un select con canales para permitir interrupciones inmediatas.
