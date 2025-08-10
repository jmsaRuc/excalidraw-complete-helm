FROM node:18 AS build

WORKDIR /home/node/app

COPY . .

RUN echo 'VITE_APP_BACKEND_V2_GET_URL="$VITE_FRONTEND_URL/api/v2/"' >> .env.production
RUN echo 'VITE_APP_BACKEND_V2_POST_URL="$VITE_FRONTEND_URL/api/v2/post/"' >> .env.production
RUN echo 'VITE_APP_LIBRARY_URL="https://libraries.excalidraw.com"' >> .env.production
RUN echo 'VITE_APP_LIBRARY_BACKEND="$VITE_FRONTEND_URL/libraries"' >> .env.production
RUN echo 'VITE_APP_AI_BACKEND="$VITE_FRONTEND_URL/ai/"' >> .env.production
RUN echo 'VITE_APP_PLUS_LP="$VITE_FRONTEND_URL/plus/"' >> .env.production
RUN echo 'VITE_APP_PLUS_APP="$VITE_FRONTEND_URL/app/"' >> .env.production
RUN echo 'VITE_APP_WS_SERVER_URL="$VITE_FRONTEND_URL"' >> .env.production
RUN echo 'VITE_APP_FIREBASE_CONFIG={"apiKey":"AIzaSyAd15pYlMci_xIp9ko6wkEsDzAAA0Dn0RU","authDomain":"","databaseURL":"","projectId":"excalidraw-room-persistence","storageBucket":"","messagingSenderId":"654800341332","appId":"1:654800341332:web:4a692de832b55bd57ce0c1"}' >> .env.production
RUN echo 'VITE_APP_DISABLE_TRACKING=yes' >> .env.production
RUN echo 'NODE_ENV="production"' >> .env.production

RUN cat .env.production

RUN npm install
RUN cd excalidraw-app && npm run build:app:docker

FROM ubuntu:20.04
COPY --from=build /home/node/app/excalidraw-app/build /frontend

