import { useAuth } from '../context/AuthContext';

export function Header() {
  const { user, logout } = useAuth();
  // Only absolute http(s) URLs are safe to use as an <img> source; relative or
  // empty values would render a broken image for local accounts.
  const avatarURL =
    user?.avatar_url && /^https?:\/\//i.test(user.avatar_url) ? user.avatar_url : '';

  return (
    <header className="header">
      <h1 className="header__logo">TodoDemo</h1>
      <div className="header__user">
        {user && (
          <>
            {avatarURL && (
              <img
                src={avatarURL}
                alt={user.display_name}
                className="header__avatar"
              />
            )}
            <span className="header__name">{user.display_name}</span>
            <button onClick={logout} className="header__logout-btn">
              Logout
            </button>
          </>
        )}
      </div>
    </header>
  );
}
