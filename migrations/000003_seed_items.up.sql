INSERT INTO items (id, name, total_stock, reserved_stock) 
VALUES 
    ('item_4021', 'Flash Sale Smartphone X', 100, 0),
    ('item_4022', 'Limited Edition Wireless Earbuds', 50, 0)
ON CONFLICT (id) DO NOTHING;
