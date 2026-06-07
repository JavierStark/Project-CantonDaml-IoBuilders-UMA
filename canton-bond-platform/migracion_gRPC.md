# Documentación de la Migración del Listener a gRPC Nativo

Este componente funciona como una pasarela intermedia de datos dentro de nuestra plataforma de bonos. Su propósito fundamental es capturar los eventos y cambios de estado generados en el libro de registro de la blockchain (Canton) y retransmitirlos de forma inmediata hacia la interfaz de usuario (Frontend) utilizando conexiones WebSocket bidireccionales en tiempo real.

---

## 1. Cambios Realizados y su Explicación Arquitectónica

Para comprender la reestructuración del componente, a continuación se detallan los cambios aplicados en la arquitectura explicados desde la perspectiva del flujo de datos:

### Transición de Consultas Web (HTTP) a Conexión gRPC Directa
* **Cómo funcionaba antes:** El Listener intentaba interactuar con la blockchain simulando peticiones web tradicionales (HTTP/REST). Esto requería que el servicio solicitara información de manera intermitente, actuando como un cliente externo común que dependía de traductores intermedios de protocolo.
* **Cómo funciona ahora:** Se ha eliminado la capa web intermedia. El Listener ahora abre un canal de comunicación directo utilizando **gRPC nativo**. Esto significa que el servicio se comunica utilizando el lenguaje e infraestructura internos de la propia red de Canton, conectándose directamente al puerto de datos del nodo participante sin necesidad de intermediarios.

### Sustitución del Mecanismo de Polling por Streaming Asíncrono
* **El problema del Polling:** Bajo el esquema anterior, para simular el tiempo real, el sistema debía preguntar repetidamente a la blockchain en intervalos cortos de tiempo si se había producido una nueva transacción (por ejemplo, la acuñación de un bono o *mint*). Esta técnica generaba una sobrecarga constante de la red, un alto consumo de CPU y latencias elevadas.
* **La solución con Streaming:** gRPC se ejecuta sobre HTTP/2, un protocolo que permite mantener conexiones persistentes abiertas en ambos sentidos. Con la migración, el Listener realiza una única solicitud de suscripción al arrancar. El canal se queda abierto en un estado pasivo y es el propio nodo participante de Canton el que empuja el bloque binario de la transacción en el milisegundo exacto en que se valida en el registro, reduciendo la latencia a cero y optimizando el ancho de banda.

### Implementación del Filtro Comodín Obligatorio (`WildcardFilter`)
* **Comportamiento nativo de Canton v3:** En las versiones modernas de la API de Ledger de Daml, cuando un cliente abre un flujo de datos indicando simplemente a qué entidad (*Party*) representa, el nodo por defecto no envía ninguna información por razones de optimización. La conexión se establece con éxito, pero el canal permanece vacío.
* **La corrección técnica:** Se modificó la estructura interna de la petición gRPC para inyectar explitamente un filtro acumulativo de tipo comodín (*Wildcard*). Esta instrucción técnica le comunica explícitamente al nodo que el Listener requiere la transmisión activa de todos los contratos y eventos de creación o archivado asociados a los derechos de lectura de esa entidad.

### Control de Tolerancia a Fallos en el Arranque Cooperativo
* **El conflicto de tiempos en Docker:** Al iniciar el entorno con Docker Compose, todos los servicios (nodos de Canton, bases de datos, Backend y Listener) reciben la orden de encendido simultáneamente. El Listener en Go se compila e inicia en milisegundos, mientras que los participantes de la blockchain tardan habitualmente entre uno y dos minutos en verificar su criptografía e inicializar sus servicios internos. Esto provocaba que el Listener encontrara el puerto cerrado, generara un error crítico de conexión rechazada y se apagara de forma definitiva.
* **La solución de resiliencia:** Se envolvió el proceso de conexión gRPC dentro de una estructura lógica iterativa infinita controlada por temporizadores. Si el puerto gRPC está cerrado, el componente intercepta el error, detiene la ejecución de forma segura durante 5 segundos y vuelve a intentar la conexión de manera indefinida, garantizando que el sistema se enlace automáticamente en cuanto la blockchain esté operativa.

---

## 2. Errores de Compatibilidad Encontrados (Canton, Daml y gRPC)

Durante el proceso de desarrollo y migración nos topamos con barreras críticas de compatibilidad debido a la evolución de las herramientas de Daml y Canton (especialmente al pasar a Canton v3 y su API de Ledger v2). A continuación se detallan estos fallos para comprender el porqué de la estructura actual del código:

### El error del "Stream Silencioso" (Filtros por Party Vacíos)
* **El problema:** Al mapear la Party en el código inicial, el campo `Cumulative` se enviaba como una lista vacía (`[]*pb.CumulativeFilter{}`). En versiones antiguas de Daml esto bastaba para escuchar todo, pero en Canton moderno esto rompe la compatibilidad. El servidor acepta la conexión (devuelve un código HTTP 200 / gRPC OK) pero entra en un estado silencioso donde **Canton decide enviar 0 eventos**.
* **La causa:** Canton v3 exige que si te suscribes a nivel de Ledger, debes especificar contractualmente qué quieres ver. Si dejas la lista vacía, el nodo asume explícitamente que tu filtro es "ningún contrato".
* **La solución:** Se tuvo que estructurar el filtro inyectando un objeto nativo `CumulativeFilter_WildcardFilter` con la propiedad `IncludeCreatedEventBlob: true` para romper esa restricción y forzar al nodo a transmitir la actividad.

### El conflicto de tipado con JSON nativo
* **El problema:** Originalmente, el sistema intentaba capturar la información directamente en texto JSON plano enviado por pasarelas HTTP previas. Al migrar a gRPC nativo, los datos que viajan por la red ya no son texto legible ni estructurado en JSON; viajan en formato **binario serializado** (Protocol Buffers).
* **La causa:** Daml maneja tipos de datos fuertemente tipados y personalizados dentro de la blockchain (como contratos que contienen estructuras complejas e identificadores largos). Intentar transformar directamente los datos nativos de gRPC a un JSON genérico en Go causaba fallos de tipado o la pérdida de campos cruciales como el cuerpo del bono (*CreatedEvent*).
* **La solución:** Se reescribió la lógica del Listener en `main.go` y `cantonledger` para actuar como un traductor manual. Ahora el Listener intercepta los punteros binarios estructurados de gRPC (`*pb.GetUpdatesResponse_Transaction`), extrae los strings esenciales (como `ContractId` y `TemplateId.EntityName`) y construye a mano un mapa compatible (`map[string]any`) que es el que finalmente se serializa en un JSON limpio y amigable para que el Frontend lo pueda entender sin esfuerzo.

---

## 3. Ficha Técnica y Especificaciones de Ingeniería

A continuación se presenta el desglose técnico y los parámetros exactos utilizados para la configuración del entorno y el comportamiento del código en Go:

### Protocolo y Red
* **Capa de Transporte:** gRPC nativo implementado sobre HTTP/2 con serialización binaria basada en Protocol Buffers (v2).
* **Puerto Destino:** Puerto `5011` del contenedor `participant1` (Puerto asignado para la API de Ledger externa de Canton).
* **Mecanismo de Seguridad:** Credenciales inseguras explícitas (`insecure.NewCredentials()`) para comunicaciones dentro de la red interna aislada de Docker.

### Configuración del Objeto de Suscripción (`GetUpdatesRequest`)
* **Punto de Inicio (`BeginExclusive`):** Configurado en `0` (origen del Ledger) para asegurar el procesamiento ordenado de eventos.
* **Estructura del Filtro (`FiltersByParty`):** Mapeo clave-valor indexado por el string dinámico de la variable `LISTENER_PARTY`.
* **Filtro de Datos:** `CumulativeFilter` que contiene una instancia de `WildcardFilter` con la propiedad técnica `IncludeCreatedEventBlob: true`. Este parámetro es estrictamente necesario para deserializar contratos complejos bajo las especificaciones de Canton v3.
* **Formato de Salida (`TransactionShape`):** Definido bajo la constante enum `TRANSACTION_SHAPE_LEDGER_EFFECTS`, lo que instruye al nodo a empaquetar de forma explícita las consecuencias transaccionales (altas y bajas de contratos) en lugar del árbol de ejecución completo de la transacción.

### Arquitectura de Software en Go
* **Concurrencia Asíncrona:** El software hace uso de Goroutines independientes. El flujo gRPC opera en su propio hilo ligero bloqueante (`stream.Recv()`), mientras que el servidor de WebSockets se ejecuta en paralelo bajo el motor HTTP nativo de Go en el puerto `8081`.
* **Sincronización de Memoria (Thread-Safety):** Dado que múltiples usuarios del Frontend pueden conectar sus navegadores al WebSocket al mismo tiempo, el mapa global de clientes (`wsClients`) constituye un recurso compartido propenso a condiciones de carrera (*race conditions*). Para evitar corrupción de memoria al propagar masivamente un evento de *mint*, se utiliza un mecanismo de exclusión mutua (`sync.Mutex`) que bloquea y libera de forma segura la estructura durante la iteración de envío de datos.

---

## 4. Comandos de Terminal para Ejecución, Despliegue y Pruebas

Para validar el correcto funcionamiento del componente migrado y verificar el flujo en tiempo real, ejecute los siguientes comandos estrictamente en el orden indicado:

### Paso 1: Posicionarse en el directorio raíz del proyecto
Abra su terminal y asegúrese de estar situado en la raíz de la plataforma de bonos donde reside el archivo de configuración de Docker:
```bash
cd ~/blosein/Project-CantonDaml-IoBuilders-UMA/canton-bond-platform
```

### Paso 2: Limpieza absoluta del entorno previo
Este comando detiene todos los servicios concurrentes y elimina los contenedores, redes y elementos huérfanos generados por ejecuciones anteriores para asegurar un despliegue libre de datos corruptos en memoria:
```bash
docker compose down --remove-orphans

```

### Paso 3: Compilación del código y lanzamiento de la plataforma
Este comando fuerza a Docker a leer los archivos de Go modificados dentro de las carpetas locales, compila el ejecutable del nuevo Listener binario dentro de su respectiva imagen alpina y levanta todos los componentes de la arquitectura de forma unificada:

```bash
docker compose up --build
```

### Paso 4: Monitorización y auditoría del canal gRPC (En otra terminal)
Para observar el comportamiento interno del Listener y verificar si se ha enganchado correctamente a la blockchain, abra una pestaña o ventana secundaria de su terminal, acceda al mismo directorio del proyecto y ejecute el visor de logs en tiempo real filtrado por este componente:

```bash
docker compose logs -f listener
```
