# Code Challenge para YUNO

## Descripción

Este proyecto es un reto técnico para YUNO. Consiste en una aplicación para un restaurante que gestiona pedidos recibidos por teléfono, en el local o mediante delivery. El desafío se centra en manejar los pedidos de los clientes de forma ordenada, gestionando la concurrencia de manera eficiente.

Para lograr esto, se implementaron dos colas de pedidos: una para pedidos VIP y otra para pedidos normales. Cada tipo de pedido (VIP y no VIP) cuenta con colas específicas para cada estado del pedido: **pendiente** y **en preparación**.

### Diagrama
![Diagrama de la aplicación](./GourmetVelozDiagrama.png)

## Cómo ejecutar

Para ejecutar la aplicación, sigue estos pasos:

1. Tener Docker y Docker Compose instalados.
2. En la raíz del proyecto, ejecuta el siguiente comando:
   ```bash
   docker-compose up -d
   ```
3. Utilizar la colección de Postman en la raiz del proyecto para hacer las solicitudes
