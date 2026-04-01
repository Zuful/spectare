build:
	cd frontend && npm install && npm run build && cd ..
	/usr/local/go/bin/go build -o spectare .

dev-backend:
	/usr/local/go/bin/go run .

dev-frontend:
	cd frontend && npm run dev

clean:
	rm -f spectare && rm -rf frontend/out frontend/.next
