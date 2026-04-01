build:
	cd frontend && npm install && npm run build && cd ..
	go build -o spectare .

dev-backend:
	go run .

dev-frontend:
	cd frontend && npm run dev

clean:
	rm -f spectare && rm -rf frontend/out frontend/.next
