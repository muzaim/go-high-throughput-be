INSERT INTO items (id, name, total_stock, reserved_stock) 
VALUES 
    ('item_4021', 'iPhone 18 Pro Max 1TB', 100, 0),
    ('item_4022', 'MacBook Pro M4 Max 16"', 50, 0),
    ('item_4023', 'iPad Pro OLED 13" M4', 40, 0),
    ('item_4024', 'Apple Watch Ultra 3', 60, 0),
    ('item_4025', 'AirPods Max 2 USB-C', 75, 0),
    ('item_4026', 'Sony PlayStation 5 Pro', 30, 0),
    ('item_4027', 'LG UltraGear OLED 4K Monitor', 25, 0),
    ('item_4028', 'Custom Mechanical Keyboard RGB', 150, 0)
ON CONFLICT (id) DO NOTHING;
