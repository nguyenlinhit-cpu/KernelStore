-- 000001_initial_schema.down.sql
-- Xóa toàn bộ bảng theo thứ tự FK ngược lại.

DROP TABLE IF EXISTS "__EFMigrationsHistory";
DROP TABLE IF EXISTS "WarrantyClaims";
DROP TABLE IF EXISTS "ChatMessages";
DROP TABLE IF EXISTS "Conversations";
DROP TABLE IF EXISTS "RefreshTokens";
DROP TABLE IF EXISTS "CartItems";
DROP TABLE IF EXISTS "Reviews";
DROP TABLE IF EXISTS "OrderDetails";
DROP TABLE IF EXISTS "Orders";
DROP TABLE IF EXISTS "ProductImages";
DROP TABLE IF EXISTS "Products";
DROP TABLE IF EXISTS "Categories";
DROP TABLE IF EXISTS "Shops";
DROP TABLE IF EXISTS "Addresses";
DROP TABLE IF EXISTS "AspNetUserTokens";
DROP TABLE IF EXISTS "AspNetUserRoles";
DROP TABLE IF EXISTS "AspNetUserLogins";
DROP TABLE IF EXISTS "AspNetUserClaims";
DROP TABLE IF EXISTS "AspNetRoleClaims";
DROP TABLE IF EXISTS "AspNetRoles";
DROP TABLE IF EXISTS "AspNetUsers";
