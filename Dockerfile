
# Imagen mínima sin sistema operativo
FROM scratch

WORKDIR /app

# Copiar el ejecutable compilado localmente
COPY micro-test .

# Puerto del microservicio
EXPOSE 8089

# Iniciar aplicación
ENTRYPOINT ["./micro-test"]
