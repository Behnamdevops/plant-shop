import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getProfile } from '../api/account'
import { listAddresses } from '../api/addresses'
import type { Profile } from '../api/account'
import type { Address } from '../api/addresses'
import { useAuth } from '../hooks/useAuth'

export default function AccountPage() {
  const { user } = useAuth()
  const [profile, setProfile] = useState<Profile | null>(null)
  const [addresses, setAddresses] = useState<Address[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!user) return

    let ignore = false

    Promise.all([getProfile(), listAddresses()])
      .then(([profileData, addressesData]) => {
        if (!ignore) {
          setProfile(profileData)
          setAddresses(addressesData)
        }
      })
      .catch((err) => {
        if (!ignore) {
          setError(err instanceof Error ? err.message : 'مشکلی در بارگذاری اطلاعات حساب پیش آمد')
        }
      })
      .finally(() => {
        if (!ignore) setLoading(false)
      })

    return () => {
      ignore = true
    }
  }, [user])

  if (!user) {
    return (
      <main>
        <h1>حساب کاربری</h1>
        <p className="empty-state">
          برای مشاهده حساب کاربری، <Link to="/login">وارد شوید</Link>.
        </p>
      </main>
    )
  }

  if (loading) {
    return (
      <main>
        <h1>حساب کاربری</h1>
        <p className="state-message">در حال بارگذاری...</p>
      </main>
    )
  }

  if (error) {
    return (
      <main>
        <h1>حساب کاربری</h1>
        <p className="alert alert-error" role="alert">
          {error}
        </p>
      </main>
    )
  }

  const defaultAddress = addresses.find(addr => addr.is_default)

  return (
    <main>
      <h1>حساب کاربری</h1>
      
      <div className="account-layout">
        <aside className="account-sidebar">
          <nav className="account-nav">
            <h3 className="account-nav-title">مدیریت حساب</h3>
            <ul>
              <li className="account-nav-item active">
                <Link to="/account">نمای کلی</Link>
              </li>
              <li className="account-nav-item">
                <Link to="/account/profile">اطلاعات شخصی</Link>
              </li>
              <li className="account-nav-item">
                <Link to="/account/addresses">آدرس‌ها</Link>
              </li>
              <li className="account-nav-item">
                <Link to="/orders">سفارش‌ها</Link>
              </li>
            </ul>
          </nav>
        </aside>

        <div className="account-content">
          <div className="account-welcome">
            <h2>خوش آمدید، {profile?.name}!</h2>
            <p>از این بخش می‌توانید حساب کاربری خود را مدیریت کنید.</p>
          </div>

          <div className="account-cards">
            <div className="account-card">
              <h3>اطلاعات شخصی</h3>
              <div className="account-card-content">
                <p>
                  <strong>نام:</strong> {profile?.name}
                </p>
                <p>
                  <strong>ایمیل:</strong> {profile?.email}
                </p>
                <p>
                  <strong>شماره تماس:</strong> {profile?.phone || 'ثبت نشده'}
                </p>
              </div>
              <div className="account-card-actions">
                <Link to="/account/profile" className="btn btn-secondary btn-sm">
                  ویرایش اطلاعات
                </Link>
              </div>
            </div>

            <div className="account-card">
              <h3>آدرس پیش‌فرض</h3>
              <div className="account-card-content">
                {defaultAddress ? (
                  <>
                    <p>
                      <strong>عنوان:</strong> {defaultAddress.label || 'بدون عنوان'}
                    </p>
                    <p>
                      <strong>گیرنده:</strong> {defaultAddress.recipient_name}
                    </p>
                    <p>
                      <strong>آدرس:</strong> {defaultAddress.address_line1}
                      {defaultAddress.address_line2 && `، ${defaultAddress.address_line2}`}
                    </p>
                    <p>
                      <strong>شهر:</strong> {defaultAddress.city}، کد پستی: {defaultAddress.postal_code}
                    </p>
                  </>
                ) : (
                  <p className="empty-state">آدرس پیش‌فرضی ثبت نشده است.</p>
                )}
              </div>
              <div className="account-card-actions">
                <Link to="/account/addresses" className="btn btn-secondary btn-sm">
                  مدیریت آدرس‌ها
                </Link>
              </div>
            </div>

            <div className="account-card">
              <h3>آخرین سفارش‌ها</h3>
              <div className="account-card-content">
                <p>برای مشاهده تاریخچه سفارش‌ها و وضعیت آن‌ها، به بخش سفارش‌ها مراجعه کنید.</p>
              </div>
              <div className="account-card-actions">
                <Link to="/orders" className="btn btn-secondary btn-sm">
                  مشاهده سفارش‌ها
                </Link>
              </div>
            </div>
          </div>
        </div>
      </div>
    </main>
  )
}