package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KernelStore/backend-go/internal/identity"
	"github.com/KernelStore/backend-go/internal/models"
	"github.com/KernelStore/backend-go/internal/money"
	"github.com/KernelStore/backend-go/internal/repository"
	"github.com/KernelStore/backend-go/internal/services"
)

// Mật khẩu chung của các tài khoản seller demo.
const demoSellerPassword = "Seller@12345"

// Sản phẩm thêm cuối cùng — có nó nghĩa là lần seed trước đã chạy trọn vẹn.
const demoMarkerSlug = "github-copilot-1-year"

// seedDemoData tạo dữ liệu demo: 10 category, 7 seller + shop đã duyệt, 57 sản phẩm
// (ảnh nằm sẵn trong uploads/<slug>.jpg). Tương đương DatabaseSeeder.SeedDemoDataAsync.
//
// Idempotent: đã có sản phẩm marker thì bỏ qua; nếu lần trước chạy dở thì chạy lại,
// mọi bước đều "có rồi thì dùng lại", nên không sinh bản ghi trùng.
func seedDemoData(ctx context.Context, pool *pgxpool.Pool) error {
	done, err := repository.ProductSlugTaken(ctx, pool, demoMarkerSlug)
	if err != nil {
		return err
	}
	if done {
		fmt.Println("[seed] demo data already present — skipping.")
		return nil
	}

	err = repository.InTx(ctx, pool, func(tx pgx.Tx) error {
		// ── Categories (tạo nếu chưa có, theo slug) ──────────────────────
		cats := map[string]uuid.UUID{}
		for _, c := range demoCategories {
			var parent *uuid.UUID
			if c.parent != "" {
				id := cats[c.parent]
				parent = &id
			}
			id, err := ensureCategory(ctx, tx, c.name, c.slug, parent)
			if err != nil {
				return err
			}
			cats[c.key] = id
		}

		// ── Sellers + shop đã duyệt ──────────────────────────────────────
		shops := map[string]uuid.UUID{}
		for _, s := range demoShops {
			sellerID, err := ensureSeller(ctx, tx, s.email, s.userName, s.fullName)
			if err != nil {
				return err
			}
			id, err := ensureShop(ctx, tx, sellerID, s.name, s.slug, s.description)
			if err != nil {
				return err
			}
			shops[s.key] = id
		}

		// ── Products ─────────────────────────────────────────────────────
		for _, p := range demoProducts {
			if err := ensureProduct(ctx, tx, shops[p.shop], cats[p.category], p); err != nil {
				return fmt.Errorf("sản phẩm %s: %w", p.slug, err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Println("[seed] demo data created: 10 categories, 7 shops, 57 products.")
	return nil
}

func ensureCategory(ctx context.Context, db repository.DBTX, name, slug string, parent *uuid.UUID) (uuid.UUID, error) {
	existing, err := repository.FindCategoryBySlug(ctx, db, slug)
	if err != nil || existing != nil {
		return idOf(existing), err
	}
	c := &models.Category{ID: services.NewUUID(), Name: name, Slug: slug, ParentID: parent}
	return c.ID, repository.InsertCategory(ctx, db, c)
}

func idOf(c *repository.CategoryWithCount) uuid.UUID {
	if c == nil {
		return uuid.Nil
	}
	return c.ID
}

// ensureSeller tạo tài khoản Seller (qua đúng luồng tạo user như đăng ký) nếu email chưa có.
func ensureSeller(ctx context.Context, db repository.DBTX, email, userName, fullName string) (uuid.UUID, error) {
	existing, err := repository.FindUserByEmail(ctx, db, identity.Normalize(email))
	if err != nil || existing != nil {
		if existing != nil {
			return existing.ID, nil
		}
		return uuid.Nil, err
	}
	u := &models.ApplicationUser{
		ID: services.NewUUID(), UserName: &userName, Email: &email, FullName: fullName,
		Role: models.UserRoleSeller, IsActive: true, CreatedAt: services.Now(),
	}
	errs, err := services.CreateUser(ctx, db, u, demoSellerPassword)
	if err != nil {
		return uuid.Nil, err
	}
	if len(errs) > 0 {
		return uuid.Nil, fmt.Errorf("[seed] failed to create seller %s: %s", email, strings.Join(errs, "; "))
	}
	_, err = repository.AddToRole(ctx, db, u.ID, "SELLER")
	return u.ID, err
}

func ensureShop(ctx context.Context, db repository.DBTX, ownerID uuid.UUID, name, slug, description string) (uuid.UUID, error) {
	var id uuid.UUID
	err := db.QueryRow(ctx, `SELECT "Id" FROM "Shops" WHERE "Slug" = $1`, slug).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != pgx.ErrNoRows {
		return uuid.Nil, err
	}
	s := &models.Shop{
		ID: services.NewUUID(), Name: name, Slug: slug, Description: description,
		Status: models.ShopStatusApproved, CreatedAt: services.Now(), OwnerID: ownerID,
	}
	return s.ID, repository.InsertShop(ctx, db, s)
}

func ensureProduct(ctx context.Context, db repository.DBTX, shopID, categoryID uuid.UUID, d demoProduct) error {
	if taken, err := repository.ProductSlugTaken(ctx, db, d.slug); err != nil || taken {
		return err
	}
	var sale *money.Money
	if d.salePrice != "" {
		m := money.MustParse(d.salePrice)
		sale = &m
	}
	p := &models.Product{
		ID: services.NewUUID(), Name: d.name, Slug: d.slug, Description: d.description,
		Price: money.MustParse(d.price), SalePrice: sale, StockQuantity: d.stock, Sku: d.sku,
		WarrantyMonths: 12, IsActive: true, CreatedAt: services.Now(),
		ShopID: shopID, CategoryID: &categoryID,
	}
	if err := repository.InsertProduct(ctx, db, p); err != nil {
		return err
	}
	return repository.InsertProductImages(ctx, db, []models.ProductImage{{
		ID: services.NewUUID(), Url: "http://localhost:5000/uploads/" + d.slug + ".jpg",
		AltText: d.name, IsPrimary: true, DisplayOrder: 0, ProductID: p.ID,
	}})
}

// ─── Dữ liệu demo (chép nguyên từ DatabaseSeeder.cs) ─────────────────────────

var demoCategories = []struct{ key, name, slug, parent string }{
	{"electronics", "Electronics", "electronics", ""},
	{"laptops", "Laptops", "laptops", "electronics"},
	{"phones", "Phones", "phones", "electronics"},
	{"tablets", "Tablets", "tablets", "electronics"},
	{"accessories", "Accessories", "accessories", ""},
	// Category chuyên ngành IT (cấp gốc để hiện thành khu riêng ở trang chủ).
	{"iot", "IoT & Embedded", "iot", ""},
	{"aiml", "AI & Machine Learning", "ai-ml", ""},
	{"security", "Cybersecurity", "security", ""},
	{"sysadmin", "SysAdmin & DevOps", "sysadmin", ""},
	{"developer", "Developer Tools", "developer", ""},
}

var demoShops = []struct{ key, email, userName, fullName, name, slug, description string }{
	{"shopAlice", "seller1@demo.ks", "demo_alice", "Alice Nguyen", "TechWorld Store", "techworld-store",
		"Authentic laptops, smartphones and tablets from top brands."},
	{"shopBob", "seller2@demo.ks", "demo_bob", "Bob Tran", "GadgetHub", "gadgethub",
		"Audio, smart speakers, chargers and mobile accessories."},
	{"shopIot", "iot@demo.ks", "demo_iot", "Carol Pham", "IoT Depot", "iot-depot",
		"Single-board computers, microcontrollers, sensors and gateways for makers and embedded engineers."},
	{"shopAi", "ai@demo.ks", "demo_ai", "David Le", "Neural Forge", "neural-forge",
		"GPUs, edge accelerators and workstations built for training and deploying AI models."},
	{"shopSec", "security@demo.ks", "demo_sec", "Emma Vo", "SecOps Armory", "secops-armory",
		"Hardware keys, pentest gadgets and security tooling for red and blue teams."},
	{"shopOps", "sysadmin@demo.ks", "demo_ops", "Frank Do", "OpsCenter", "opscenter",
		"Networking, rackmount servers, NAS and power gear to run reliable infrastructure."},
	{"shopDev", "developer@demo.ks", "demo_dev", "Grace Ha", "DevTools Hub", "devtools-hub",
		"Keyboards, monitors, docks and software licenses that power a developer's workflow."},
}

type demoProduct struct {
	shop, category, name, slug, description string
	price, salePrice                        string // salePrice "" = null
	stock                                   int
	sku                                     string
}

// Thứ tự giữ nguyên như C# (CreatedAt tăng dần → "featured" hiện sản phẩm thêm sau cùng trước).
var demoProducts = []demoProduct{
	{"shopAlice", "laptops", "Apple MacBook Pro 14 Inch Space Grey", "apple-macbook-pro-14-inch-space-grey",
		"The MacBook Pro 14 Inch in Space Grey is a powerful and sleek laptop, featuring Apple's M1 Pro chip for exceptional performance and a stunning Retina display.",
		"1999.99", "1906.19", 24, "LAP-APP-APP-078"},
	{"shopAlice", "laptops", "Asus Zenbook Pro Dual Screen Laptop", "asus-zenbook-pro-dual-screen-laptop",
		"The Asus Zenbook Pro Dual Screen Laptop is a high-performance device with dual screens, providing productivity and versatility for creative professionals.",
		"1799.99", "1599.47", 45, "LAP-ASU-ASU-079"},
	{"shopAlice", "laptops", "Huawei Matebook X Pro", "huawei-matebook-x-pro",
		"The Huawei Matebook X Pro is a slim and stylish laptop with a high-resolution touchscreen display, offering a premium experience for users on the go.",
		"1399.99", "1268.67", 75, "LAP-HUA-HUA-080"},
	{"shopAlice", "laptops", "Lenovo Yoga 920", "lenovo-yoga-920",
		"The Lenovo Yoga 920 is a 2-in-1 convertible laptop with a flexible hinge, allowing you to use it as a laptop or tablet, offering versatility and portability.",
		"1099.99", "1027.94", 40, "LAP-LEN-LEN-081"},
	{"shopAlice", "laptops", "New DELL XPS 13 9300 Laptop", "new-dell-xps-13-9300-laptop",
		"The New DELL XPS 13 9300 Laptop is a compact and powerful device, featuring a virtually borderless InfinityEdge display and high-end performance for various...",
		"1499.99", "1321.64", 74, "LAP-DEL-DEL-082"},
	{"shopAlice", "phones", "iPhone 5s", "iphone-5s",
		"The iPhone 5s is a classic smartphone known for its compact design and advanced features during its release. While it's an older model, it still provides a r...",
		"199.99", "174.17", 25, "SMA-APP-IPH-121"},
	{"shopAlice", "phones", "iPhone 6", "iphone-6",
		"The iPhone 6 is a stylish and capable smartphone with a larger display and improved performance. It introduced new features and design elements, making it a...",
		"299.99", "279.92", 60, "SMA-APP-IPH-122"},
	{"shopAlice", "phones", "iPhone 13 Pro", "iphone-13-pro",
		"The iPhone 13 Pro is a cutting-edge smartphone with a powerful camera system, high-performance chip, and stunning display. It offers advanced features for us...",
		"1099.99", "996.92", 56, "SMA-APP-IPH-123"},
	{"shopAlice", "phones", "iPhone X", "iphone-x",
		"The iPhone X is a flagship smartphone featuring a bezel-less OLED display, facial recognition technology (Face ID), and impressive performance. It represents...",
		"899.99", "723.68", 37, "SMA-APP-IPH-124"},
	{"shopAlice", "phones", "Oppo A57", "oppo-a57",
		"The Oppo A57 is a mid-range smartphone known for its sleek design and capable features. It offers a balance of performance and affordability, making it a pop...",
		"249.99", "", 19, "SMA-OPP-OPP-125"},
	{"shopAlice", "phones", "Oppo F19 Pro Plus", "oppo-f19-pro-plus",
		"The Oppo F19 Pro Plus is a feature-rich smartphone with a focus on camera capabilities. It boasts advanced photography features and a powerful performance fo...",
		"399.99", "325.43", 78, "SMA-OPP-OPP-126"},
	{"shopAlice", "tablets", "iPad Mini 2021 Starlight", "ipad-mini-2021-starlight",
		"The iPad Mini 2021 in Starlight is a compact and powerful tablet from Apple. Featuring a stunning Retina display, powerful A-series chip, and a sleek design,...",
		"499.99", "481.79", 47, "TAB-APP-IPA-159"},
	{"shopAlice", "tablets", "Samsung Galaxy Tab S8 Plus Grey", "samsung-galaxy-tab-s8-plus-grey",
		"The Samsung Galaxy Tab S8 Plus in Grey is a high-performance Android tablet by Samsung. With a large AMOLED display, powerful processor, and S Pen support, i...",
		"599.99", "520.13", 62, "TAB-SAM-SAM-160"},
	{"shopAlice", "tablets", "Samsung Galaxy Tab White", "samsung-galaxy-tab-white",
		"The Samsung Galaxy Tab in White is a sleek and versatile Android tablet. With a vibrant display, long-lasting battery, and a range of features, it offers a g...",
		"349.99", "286.29", 92, "TAB-SAM-SAM-161"},
	{"shopBob", "accessories", "Amazon Echo Plus", "amazon-echo-plus",
		"The Amazon Echo Plus is a smart speaker with built-in Alexa voice control. It features premium sound quality and serves as a hub for controlling smart home d...",
		"99.99", "87.92", 61, "MOB-AMA-AMA-099"},
	{"shopBob", "accessories", "Apple Airpods", "apple-airpods",
		"The Apple Airpods offer a seamless wireless audio experience. With easy pairing, high-quality sound, and Siri integration, they are perfect for on-the-go lis...",
		"129.99", "109.79", 67, "MOB-APP-APP-100"},
	{"shopBob", "accessories", "Apple AirPods Max Silver", "apple-airpods-max-silver",
		"The Apple AirPods Max in Silver are premium over-ear headphones with high-fidelity audio, adaptive EQ, and active noise cancellation. Experience immersive so...",
		"549.99", "474.81", 59, "MOB-APP-APP-101"},
	{"shopBob", "accessories", "Apple Airpower Wireless Charger", "apple-airpower-wireless-charger",
		"The Apple AirPower Wireless Charger provides a convenient way to charge your compatible Apple devices wirelessly. Simply place your devices on the charging m...",
		"79.99", "76.41", 1, "MOB-APP-APP-102"},
	{"shopBob", "accessories", "Apple HomePod Mini Cosmic Grey", "apple-homepod-mini-cosmic-grey",
		"The Apple HomePod Mini in Cosmic Grey is a compact smart speaker that delivers impressive audio and integrates seamlessly with the Apple ecosystem for a smar...",
		"99.99", "81.89", 27, "MOB-APP-APP-103"},
	{"shopBob", "accessories", "Apple iPhone Charger", "apple-iphone-charger",
		"The Apple iPhone Charger is a high-quality charger designed for fast and efficient charging of your iPhone. Ensure your device stays powered up and ready to go.",
		"19.99", "16.29", 31, "MOB-APP-APP-104"},
	{"shopIot", "iot", "Raspberry Pi 5 8GB", "raspberry-pi-5-8gb",
		"The Raspberry Pi 5 with 8GB RAM is a credit-card sized computer powered by a quad-core Cortex-A76 CPU. Ideal for edge computing, home labs and IoT gateways.",
		"89.99", "82.99", 120, "IOT-RPI-RP5-201"},
	{"shopIot", "iot", "ESP32 DevKit V1", "esp32-devkit-v1",
		"The ESP32 DevKit V1 is a low-cost Wi-Fi + Bluetooth microcontroller board, perfect for connected sensors, home automation and battery-powered IoT nodes.",
		"12.99", "9.99", 300, "IOT-ESP-E32-202"},
	{"shopIot", "iot", "Arduino Uno R4 WiFi", "arduino-uno-r4-wifi",
		"The Arduino Uno R4 WiFi pairs a 32-bit Renesas MCU with an ESP32-S3 radio and a built-in LED matrix — a friendly board for learning embedded and IoT.",
		"27.99", "24.50", 180, "IOT-ARD-R4W-203"},
	{"shopIot", "iot", "Raspberry Pi Pico W", "raspberry-pi-pico-w",
		"The Raspberry Pi Pico W is a tiny, ultra-affordable RP2040 microcontroller board with wireless connectivity for compact embedded projects.",
		"6.99", "", 500, "IOT-RPI-PCW-204"},
	{"shopIot", "iot", "LoRa Gateway 8-Channel", "lora-gateway-8-channel",
		"An 8-channel LoRaWAN gateway that bridges long-range, low-power sensor networks to the internet — the backbone of city-scale and agricultural IoT.",
		"159.99", "139.99", 45, "IOT-LOR-8CH-205"},
	{"shopIot", "iot", "Zigbee Smart Hub", "zigbee-smart-hub",
		"A Zigbee 3.0 smart home hub that locally controls lights, sensors and switches with low latency and no cloud dependency.",
		"49.99", "42.99", 90, "IOT-ZIG-HUB-206"},
	{"shopIot", "iot", "DHT22 Sensor Kit", "dht22-sensor-kit",
		"A DHT22 temperature and humidity sensor kit with jumper wires and resistors — a classic starting point for environmental monitoring builds.",
		"14.99", "11.99", 240, "IOT-DHT-K22-207"},
	{"shopIot", "iot", "mmWave Radar Sensor", "mmwave-radar-sensor",
		"A 60GHz mmWave presence-detection radar module that senses micro-movements for reliable room occupancy and fall detection in smart spaces.",
		"19.99", "", 160, "IOT-MMW-RAD-208"},
	{"shopAi", "aiml", "NVIDIA RTX 4090 24GB", "nvidia-rtx-4090-24gb",
		"The NVIDIA RTX 4090 with 24GB GDDR6X delivers massive throughput for training and fine-tuning deep learning models, plus blistering local inference.",
		"1799.99", "1699.99", 20, "AI-NVD-4090-301"},
	{"shopAi", "aiml", "NVIDIA Jetson Orin Nano", "nvidia-jetson-orin-nano",
		"The Jetson Orin Nano developer kit brings up to 40 TOPS of AI performance to the edge, running modern vision and robotics models in a tiny footprint.",
		"499.99", "469.99", 55, "AI-NVD-ORN-302"},
	{"shopAi", "aiml", "Google Coral USB Accelerator", "google-coral-usb-accelerator",
		"The Coral USB Accelerator adds an Edge TPU coprocessor over USB-C, running TensorFlow Lite models fast and efficiently on any host machine.",
		"59.99", "54.99", 130, "AI-GOO-COR-303"},
	{"shopAi", "aiml", "Hailo-8 AI Accelerator", "hailo-8-ai-accelerator",
		"The Hailo-8 M.2 module delivers up to 26 TOPS at remarkable power efficiency, ideal for embedding real-time neural inference into edge products.",
		"219.99", "", 40, "AI-HAI-H8A-304"},
	{"shopAi", "aiml", "Intel Neural Compute Stick 2", "intel-neural-compute-stick-2",
		"The Intel Neural Compute Stick 2 is a plug-and-play USB accelerator powered by the Movidius Myriad X VPU for prototyping deep-learning inference.",
		"99.99", "84.99", 70, "AI-INT-NCS-305"},
	{"shopAi", "aiml", "AI Workstation Threadripper", "ai-workstation-threadripper",
		"A pre-built AI workstation with an AMD Threadripper CPU, 128GB RAM and dual GPUs — ready for serious model training straight out of the box.",
		"4999.99", "4699.99", 8, "AI-WKS-TRX-306"},
	{"shopAi", "aiml", "NVIDIA A100 80GB Tensor Core", "nvidia-a100-80gb",
		"The NVIDIA A100 80GB is a data-center GPU engineered for large-scale training and high-throughput inference across demanding AI and HPC workloads.",
		"15999.99", "", 5, "AI-NVD-A100-307"},
	{"shopSec", "security", "YubiKey 5 NFC", "yubikey-5-nfc",
		"The YubiKey 5 NFC is a hardware security key supporting FIDO2, U2F, OTP and smart-card protocols for phishing-resistant multi-factor authentication.",
		"55.00", "49.00", 200, "SEC-YUB-5NF-401"},
	{"shopSec", "security", "Flipper Zero", "flipper-zero",
		"The Flipper Zero is a portable multi-tool for pentesters and hardware hackers — sub-GHz radio, RFID/NFC, infrared and GPIO in a pocket device.",
		"169.99", "159.99", 60, "SEC-FLP-ZRO-402"},
	{"shopSec", "security", "Hak5 WiFi Pineapple", "hak5-wifi-pineapple",
		"The Hak5 WiFi Pineapple is a purpose-built platform for authorized wireless auditing and rogue-AP assessments during red-team engagements.",
		"119.99", "", 35, "SEC-HAK-WPA-403"},
	{"shopSec", "security", "Proxmark3 RDV4", "proxmark3-rdv4",
		"The Proxmark3 RDV4 is the reference tool for RFID research, reading, emulating and analyzing LF and HF contactless cards and tags.",
		"299.99", "279.99", 25, "SEC-PXM-RV4-404"},
	{"shopSec", "security", "USB Rubber Ducky", "usb-rubber-ducky",
		"The USB Rubber Ducky is a keystroke-injection tool that a target sees as a keyboard — a staple for demonstrating HID attack payloads in labs.",
		"79.99", "69.99", 80, "SEC-HAK-DUK-405"},
	{"shopSec", "security", "Nitrokey HSM 2", "nitrokey-hsm-2",
		"The Nitrokey HSM 2 is an open-source hardware security module that securely generates and stores cryptographic keys for PKI and code signing.",
		"109.00", "", 50, "SEC-NTK-HS2-406"},
	{"shopSec", "security", "Faraday Signal-Blocking Bag", "faraday-signal-blocking-bag",
		"A Faraday bag that blocks cellular, Wi-Fi, GPS and RFID signals to preserve digital evidence and protect devices from remote wiping or tracking.",
		"29.99", "24.99", 150, "SEC-FRD-BAG-407"},
	{"shopOps", "sysadmin", "Ubiquiti UniFi Dream Machine", "ubiquiti-unifi-dream-machine",
		"The UniFi Dream Machine combines a security gateway, controller, switch and access point into one appliance for clean, manageable networks.",
		"379.99", "349.99", 40, "OPS-UBI-UDM-501"},
	{"shopOps", "sysadmin", "24-Port Managed Switch", "24-port-managed-switch",
		"A 24-port gigabit managed switch with VLANs, LACP and PoE budget — the workhorse of a well-segmented server rack.",
		"219.99", "199.99", 55, "OPS-NET-24S-502"},
	{"shopOps", "sysadmin", "1U Rackmount Server", "1u-rackmount-server",
		"A 1U rackmount server with a Xeon CPU, ECC memory and redundant PSUs — dense, reliable compute for virtualization and self-hosting.",
		"1299.99", "1199.99", 18, "OPS-SRV-1U-503"},
	{"shopOps", "sysadmin", "Synology 4-Bay NAS", "synology-4-bay-nas",
		"The Synology 4-Bay NAS delivers centralized storage, backups and Docker services with a polished DSM interface for homelabs and small teams.",
		"449.99", "419.99", 30, "OPS-SYN-4BN-504"},
	{"shopOps", "sysadmin", "KVM over IP Switch", "kvm-over-ip-switch",
		"A KVM-over-IP switch that gives BIOS-level remote keyboard, video and mouse access to headless servers from anywhere.",
		"189.99", "", 42, "OPS-KVM-IP-505"},
	{"shopOps", "sysadmin", "UPS 1500VA Rackmount", "ups-1500va-rackmount",
		"A 1500VA line-interactive rackmount UPS with pure sine-wave output and network monitoring to keep infrastructure alive through outages.",
		"279.99", "249.99", 36, "OPS-UPS-15R-506"},
	{"shopOps", "sysadmin", "Managed PDU Rack Strip", "managed-pdu-rack-strip",
		"A managed rack PDU with per-outlet metering and remote switching, so you can power-cycle any device in the rack over the network.",
		"159.99", "144.99", 48, "OPS-PDU-RCK-507"},
	{"shopDev", "developer", "Keychron Q1 Mechanical Keyboard", "keychron-q1-mechanical-keyboard",
		"The Keychron Q1 is a gasket-mounted, hot-swappable mechanical keyboard with a CNC aluminum body — a favorite for long coding sessions.",
		"179.99", "164.99", 90, "DEV-KEY-Q1M-601"},
	{"shopDev", "developer", "4K Dev Monitor 27-inch", "dev-monitor-4k-27",
		"A 27-inch 4K IPS monitor with USB-C power delivery and factory color calibration — crisp text and plenty of room for code and terminals.",
		"399.99", "359.99", 60, "DEV-MON-4K27-602"},
	{"shopDev", "developer", "USB-C Docking Station", "usb-c-docking-station",
		"A single-cable USB-C dock that adds dual displays, gigabit Ethernet, USB-A ports and 100W passthrough charging to any laptop.",
		"129.99", "114.99", 110, "DEV-DOK-USC-603"},
	{"shopDev", "developer", "Elgato Stream Deck MK.2", "elgato-stream-deck-mk2",
		"The Elgato Stream Deck MK.2 gives you 15 programmable LCD keys to trigger builds, run scripts and control your dev workflow at a tap.",
		"149.99", "139.99", 75, "DEV-ELG-SD2-604"},
	{"shopDev", "developer", "Ergonomic Vertical Mouse", "ergonomic-vertical-mouse",
		"An ergonomic vertical mouse that keeps your wrist in a natural handshake position to reduce strain during all-day work.",
		"39.99", "34.99", 200, "DEV-MOU-VRT-605"},
	{"shopDev", "developer", "Aluminum Laptop Stand", "aluminum-laptop-stand",
		"A sturdy aluminum laptop stand that raises your screen to eye level and improves airflow, keeping thermals and posture in check.",
		"34.99", "29.99", 180, "DEV-STD-ALU-606"},
	{"shopDev", "developer", "JetBrains All Products Pack", "jetbrains-all-products-pack",
		"A one-year individual license for the JetBrains All Products Pack — IntelliJ IDEA, PyCharm, WebStorm, Rider and every other JetBrains IDE.",
		"289.00", "", 999, "DEV-JBR-APP-607"},
	{"shopDev", "developer", "GitHub Copilot 1-Year", "github-copilot-1-year",
		"A one-year GitHub Copilot subscription — AI pair-programming that suggests whole lines and functions right inside your editor.",
		"100.00", "", 999, "DEV-GHC-1YR-608"},
}
