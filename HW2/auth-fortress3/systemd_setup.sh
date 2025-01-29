sudo bash -c "cat <<EOF > /etc/systemd/system/auth-fortress.service
[Unit]
Description=Basic API service
After=docker.service
Requires=docker.service

[Service]
Type=simple
User=ec2-user
Group=ec2-user
WorkingDirectory=/home/ec2-user/app

ExecStartPre=-/usr/bin/sudo docker rm -f go-app
ExecStartPre=/usr/bin/sudo docker build -t go-app .

ExecStart=/usr/bin/sudo docker compose up --build
ExecStop=/usr/bin/sudo docker stop go-app
ExecStopPost=/usr/bin/sudo docker rm go-app

Restart=on-failure
RestartSec=3
TimeoutStopSec=30
LimitNOFILE=4096
StandardOutput=append:/var/log/app.log
StandardError=append:/var/log/app.log

[Install]
WantedBy=multi-user.target
EOF"

sudo systemctl daemon-reload
sudo systemctl enable auth-fortress.service
sudo systemctl restart auth-fortress.service
