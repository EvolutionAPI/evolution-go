# Deploy no Easypanel

Há duas maneiras práticas de fazer o deploy do Evolution GO no Easypanel.

## Opção 1: Via Schema (Modo mais fácil)
O Easypanel possui o recurso "Create from Schema" que permite subir a aplicação com um único clique.

1. No seu Easypanel, vá até a aba **Templates** e desça até a opção **Create from Schema**.
2. Copie todo o conteúdo do arquivo `schema.json` que está nesta pasta.
3. Cole na caixa de texto do Easypanel e pressione **Create**.
4. *(Opcional)* Edite o projeto criado e vá em **Environment** no serviço `api` para alterar senhas e domínios (`GLOBAL_API_KEY`, etc).
5. Clique em **Deploy** no serviço `api` após o build!

## Opção 2: Via Docker Compose (Se você já tiver repositório conectado)
Se você conectou o Easypanel diretamente ao seu repositório GitHub e quer fazer deploy usando um arquivo Compose.

1. Crie um novo projeto/serviço do tipo **App** no Easypanel.
2. Em **Source**, selecione seu repositório no Github e marque a opção correspondente ao `Docker Compose`.
3. Defina o caminho do arquivo docker compose como: `docker/easypanel/docker-compose.yml`.
4. Defina as Environment Variables necessárias no Easypanel (como `GLOBAL_API_KEY`).
5. Clique em **Deploy**!

> Dica: Repare que o banco de dados utilizado não usa `init-db.sql`. Estamos utilizando o mesmo banco padrão criado pelo template (`postgres`) para concentrar as tabelas tanto de *auth* quanto de *users*, o que funciona perfeitamente e simplifica a nuvem.
