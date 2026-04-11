build:
	cd web && npm install && npm run build && cd ..
	cd backend && go mod tidy && go build -o spectare . && cd ..

dev-backend:
	cd backend && go run .

dev-web:
	cd web && npm run dev

clean:
	rm -f backend/spectare && rm -rf web/out web/.next
