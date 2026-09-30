-- Additive upgrade: existing products, orders and archived care metadata stay intact.
ALTER TABLE products ADD COLUMN details JSONB NOT NULL DEFAULT '{}';
ALTER TABLE products ADD CONSTRAINT product_details_object CHECK(jsonb_typeof(details)='object');
CREATE INDEX products_details_kind ON products((details->>'kind'));
CREATE INDEX products_details_brand ON products((details->>'brand'));
CREATE INDEX products_details_guide ON products((details->>'article_slug'));
INSERT INTO categories(name,slug) VALUES
('کود و تقویت‌کننده‌ها','fertilizers'),('خاک و بستر کشت','substrates'),
('ابزار و ملزومات نگهداری','tools'),('مراقبت و محافظت گیاه','protection'),
('آموزش و مشاوره','education'),('پک‌های مراقبت','bundles')
ON CONFLICT(slug) DO NOTHING;

ALTER TABLE orders DROP CONSTRAINT orders_shipping_method_check;
ALTER TABLE orders ADD CONSTRAINT orders_shipping_method_check CHECK(shipping_method IN('standard','express','digital'));
